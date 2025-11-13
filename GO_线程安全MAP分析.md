# 深度分析 sync.Map

Go 标准库里的 `sync.Map` 是一个 **为高并发场景设计的专用 map**，适合读多写少的场景。它和普通 `map + Mutex` 的设计思路完全不一样：通过 **只读区 + 脏区** 的分层结构，尽量让读操作不加锁或极少加锁，从而提升并发性能。

---

## 一、使用场景与设计目标

### 1.1 官方推荐的使用场景

`sync.Map` 并不是用来替代所有 map 的：

- 官方建议在以下场景使用：
  - **读多写少** 的场景（读远远多于写）
  - 作为 **缓存**（cache）
  - 元数据注册表（如 handler 注册表、全局配置等）
  - map 的 key 在运行期间 **不断增加**，且很少删除

如果是普通业务逻辑里小规模 map，**推荐 `map + sync.RWMutex`**，更简单、性能也不差。

---

## 二、整体结构概览

源码位置：`sync/map.go`（以下为简化版结构）

```go
type Map struct {
    mu Mutex

    read atomic.Value // readOnly

    dirty map[any]*entry
    misses int
}
```

### 2.1 read（只读区）

- `read` 是一个 `atomic.Value`，内部存的是一个 `readOnly` 结构：

```go
type readOnly struct {
    m       map[any]*entry
    amended bool
}
```

字段含义：

- `m`：只读 map
- `amended`：
  - 为 false：说明所有 key 都在 read 中
  - 为 true：说明有些 key 只存在 dirty 中（有“增量”）

### 2.2 dirty（脏区）

- `dirty` 是一个普通 `map[any]*entry`，通过 `mu` 保护
- 存储：
  - 新写入但尚未提升到 read 的 key
  - 被修改过的 key

### 2.3 entry（每个 key 对应一个 entry）

```go
type entry struct {
    p unsafe.Pointer // *any
}
```

约定：

- `p == nil`：表示已被删除（或者正在删除的中间状态）
- `p == expunged`：表示已经被完全标记为“已清除”

通过对 `p` 的原子操作，可以在无锁情况下 **安全读取**。

---

## 三、核心设计：read + dirty 双层结构

### 3.1 Read-Optimized：读优先

sync.Map 的目标：**绝大多数情况下，读不需要加锁**。

流程：

1. 先从 `read`（只读区）里查
   - 通过 `atomic.Value.Load()` 拿到只读 map
   - 直接查 `m[key]`
   - 如果找到并且 entry 有效 → 返回
   - 全程不加锁
2. 如果 read 里没找到：
   - 根据 `read.amended` 判断：
     - `false`：说明 dirty 为空或完全同步 → 直接返回不存在
     - `true`：说明有新增/变更，去 `dirty` 查找（需要加锁）

### 3.2 写流程：先 dirty，后提升到 read

- 所有写操作（Store / LoadOrStore / LoadAndDelete），都会在某些情况下写入 `dirty`
- 当访问次数达到一定条件时，会 **将 dirty 提升为新的 read**，以便后续读操作再次无锁化

---

## 四、几种核心操作的过程

### 4.1 Load(key)

简化流程：

```go
func (m *Map) Load(key any) (value any, ok bool) {
    read := m.read.Load().(readOnly)
    e, ok := read.m[key]
    if ok {
        return e.load() // 原子读取 entry
    }
    if !read.amended { // read 已完全包含全部 key
        return nil, false
    }
    // 需要去 dirty 查
    m.mu.Lock()
    defer m.mu.Unlock()
    read = m.read.Load().(readOnly)
    e, ok = read.m[key]
    if ok {
        return e.load()
    }
    e, ok = m.dirty[key]
    if !ok {
        m.missLocked()
        return nil, false
    }
    return e.load()
}
```

关键点：

- 优先在 `read` 中 **无锁** 查询
- 如果 `amended == true` 且 read 中不存在，才上锁去 dirty 查
- miss 次数会触发 `missLocked`，可能导致 read 更新

---

### 4.2 Store(key, value)

简化流程：

```go
func (m *Map) Store(key, value any) {
    read := m.read.Load().(readOnly)
    if e, ok := read.m[key]; ok {
        if e.tryStore(&value) {
            return
        }
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    // 再次从 read 读，避免期间变化
    read = m.read.Load().(readOnly)
    if e, ok := read.m[key]; ok {
        if e.unexpungeLocked() {
            // 从 expunged 状态恢复：写入 dirty
            m.dirty[key] = e
        }
        e.storeLocked(&value)
    } else if e, ok := m.dirty[key]; ok {
        e.storeLocked(&value)
    } else {
        // read 里没有，dirty 里也没有 → 初始化 dirty 再插入
        if !read.amended {
            m.dirtyLocked()
            m.read.Store(readOnly{m: read.m, amended: true})
        }
        m.dirty[key] = newEntry(value)
    }
}
```

要点：

- 优先尝试在 read 中无锁更新
- 如需要新建 key，必然走 dirty
- 如果之前从未使用过 dirty，会调用 `dirtyLocked` 将 read 的内容复制到 dirty 中，开启“增量模式”

---

### 4.3 LoadOrStore(key, value)

语义：如果 key 存在，返回原值；否则写入并返回新值。

- 先无锁地从 read 查
- 如果 read.amended == true 且未命中，再加锁去 dirty 查
- 如果都不存在，则在 dirty 里写入新 entry

---

### 4.4 LoadAndDelete(key)

- 也是优先从 read 查询
- 找到则标记 entry 为 nil（或 expunged）
- 同步处理 dirty 中的状态

---

### 4.5 Range(fn)

`Range` 的遍历策略：

1. 优先遍历 `read.m`
2. 如果存在 `dirty` 且 `read.amended == true`，再遍历 `dirty` 中那些 **不在 read 中** 的额外 key

保证：

- 每个 key 至少被访问一次
- 顺序不保证，和普通 map 一样是无序的

---

## 五、misses 与 dirty 提升机制

`Map.misses` 记录 **从 read 没找到，但实际 key 在 dirty 中找到** 的次数。

简化逻辑：

```go
func (m *Map) missLocked() {
    m.misses++
    if m.misses >= len(m.dirty) {
        // 将 dirty 提升为新的 read
        m.read.Store(readOnly{m: m.dirty})
        m.dirty = nil
        m.misses = 0
    }
}
```

含义：

- 当 `read` 太过“过时”，大部分 key 实际在 dirty 里时
- 不如直接把 `dirty` 提升为新的 `read`
- 之后读操作又可以无锁从 read 中快速命中

这就是 `sync.Map` 的核心性能优化：**自适应读写比例**。

---

## 六、entry 状态机（删除与恢复）

`entry.p` 的几种状态：

- `nil`：表示 key 被删除（但可能还在 dirty 中存在）
- `expunged`：特殊指针，表示已经彻底删除、且不会再在 read 中复活
- `有效指针`：正常存储 value

状态转换过程略复杂，但目的是让：

- 删除操作在不影响其它 goroutine 的前提下进行
- 通过原子操作读写 entry.p，保证并发安全

---

## 七、sync.Map vs map+RWMutex：性能对比思路

### 7.1 什么时候 sync.Map 更有优势？

- **读多写少**，读操作占主导时：
  - 无锁读路径高效
  - 避免 RWMutex 的大量锁竞争和 cache ping-pong

### 7.2 什么时候 map+RWMutex 更合适？

- 写比较频繁、删除很多、key 集合经常变化
- 逻辑简单，希望代码清晰可维护
- map 数据量不大（如几百、几千条）

### 7.3 一般经验：

- 优先用：`map + sync.RWMutex`
- 只有在：
  - **热点读场景**
  - 且明确存在大量并发读
  - 并且 lock 开销成为瓶颈
- 再考虑改成 `sync.Map`

---

## 八、常见使用示例

### 8.1 基本用法

```go
var m sync.Map

// 写入
m.Store("a", 1)

// 读取
if v, ok := m.Load("a"); ok {
    fmt.Println(v.(int))
}

// 只在不存在时设置
actual, loaded := m.LoadOrStore("b", 2)
fmt.Println(actual, loaded) // 如果之前有值，返回旧值且 loaded=true

// 删除
m.Delete("a")

// 遍历
m.Range(func(key, value any) bool {
    fmt.Println(key, value)
    return true // 返回 false 可以中止遍历
})
```

---

## 九、小结（记住这几点就够用了）

1. `sync.Map` 是为 **高并发、读多写少** 设计的 map
2. 底层结构 = `read (只读区)` + `dirty (脏区)` + `entry`
3. 读：
   - 优先走 read（无锁）
   - miss 且 `amended == true` 时再加锁查 dirty
4. 写：
   - 可能直接更新 read 里的 entry
   - 新 key 写入 dirty
   - 脏区随着 miss 次数达到阈值升级为新的 read
5. 删除：
   - 通过 entry.p = nil/expunged 实现，不直接删 map 元素
6. 不要把 sync.Map 当成默认 map 方案，普通业务推荐 `map + RWMutex`

---

如果你愿意，我可以：
- 再给一份 `sync.Map` 和 `map+RWMutex` 的 **benchmark 测试代码**
- 或者根据你具体的业务（比如消息路由、任务缓存、在线用户表）给出**选择建议和写法模板**。
