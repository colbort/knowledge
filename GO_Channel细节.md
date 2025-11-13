# Go Channel 细节（面试深挖版）

> 面向工程与面试：既讲原理也给“该怎么答”“怎么用”。基于 Go1.21+ 的常识与公开实现细节。

---

## 一、Channel 实现与语义（`hchan`）

### 1. 数据结构（核心字段）
- `qcount` 当前元素数量，`dataqsiz` 容量；`buf` 环形缓冲，`sendx/recvx` 读写索引。
- `closed` 标志；`lock` 互斥；
- `sendq/recvq`：等待队列（**sudog 链表**，记录阻塞的发送者或接收者）。

```
hchan {
  qcount, dataqsiz
  buf  // 环形队列: [ .... ]
  sendx, recvx
  recvq -> sudog list (等待接收)
  sendq -> sudog list (等待发送)
  lock, closed
}
```

### 2. 发送/接收/关闭的语义
- **发送（send）**：
  1) 若有 **等待接收者**（`recvq` 非空），**直接配对**：数据从发送方栈拷贝给接收者，**不入 buf**；唤醒接收方。  
  2) 否则若 **buf 未满**，写入环形缓冲；  
  3) 否则 **阻塞**，当前 G 加入 `sendq`，挂起。
- **接收（recv）**：
  1) 若有 **等待发送者**，直接从其栈拷贝数据；  
  2) 否则若 **buf 非空**，从 buf 读出；  
  3) 否则 **阻塞**，加入 `recvq`。
- **关闭（close）**：
  - 唤醒所有等待者：接收方后续读取到 **零值** 且 `ok=false`；发送到已关闭通道会 **panic**。  
  - 关闭一个 **nil** 通道会 panic；对已关闭通道再次关闭会 panic。

### 3. 零值与容量
- **无缓冲（容量 0）**：发送与接收必须**同时到达**（同步点），否则发送/接收方阻塞。
- **有缓冲**：允许生产与消费解耦；满/空时阻塞一方。

### 4. `select` 细节（公平性与非阻塞）
- **随机化轮询**：Go 对 `select` 进行**伪随机公平**选择，避免固定顺序导致饥饿（Go1.14 起改进）。
- **非阻塞操作**：`select { case ch <- v: default: }` 或 `select { case v := <-ch: default: }`。
- **超时**：`select { case <-ch: case <-time.After(d): }`（注意 `time.After` 泄漏；更推荐 `NewTimer`）。

### 5. Happens-Before（内存模型保证）
- **`send` 发生在 `recv` 之前**：发送方对数据的写入对接收方可见。  
- **`close` 发生在接收返回 `ok=false` 之前**。  
- 这些保证依赖于 channel 内部的锁与原子操作。

### 6. 常见坑/实践
- **谁关闭？** **发送方**关闭；接收方不要关（不拥有写入权）。
- **向已关闭 channel 发送** → panic；**从已关闭 channel 接收** → 立刻返回零值+`ok=false`（在清空缓冲之后）。
- **`range ch`**：读到关闭且数据耗尽时结束循环；不要依赖值判断终止。
- **nil 通道**：发送/接收会 **永久阻塞**；常用于动态禁用 `case`。
- **避免泄漏**：`time.After` 在未读时会泄漏定时器；用 `NewTimer` + `Stop` + drain。
- **大对象传递**：channel 是 **值拷贝**；传指针/索引减少复制。
- **高并发场景**：多个 goroutine 竞争同一 channel 会产生锁竞争和 cache 抖动；考虑分片（多 channel）或无锁结构。

### 7. 关键源码入口（便于背诵）
- `runtime/chan.go`：`chansend`, `chanrecv`, `closechan`，sudog 结构。  
- `runtime/proc.go`：调度循环、P/M/G 管理，work-stealing，netpoller。  
- `runtime/netpoll_*`：平台相关事件循环。

---

## 二、答题模板（30 秒版）

**Channel**：
> “`hchan` 有环形缓冲、`sendq/recvq` 等；发送优先配对唤醒等待方，其次入缓冲，满则阻塞；接收反之；**发送到已关闭 channel panic**，从已关闭读得零值且 `ok=false`；`select` 随机化避免饥饿；内存模型保证 `send→recv` 的 happens-before。”

---

## 三、实践清单（落地建议）

- **Channel**：
  - 明确“谁关闭”。设计 API 时由生产者生命周期负责 `close`；
  - 高频路径用 **多路分片 channel**（例如 2^n 个分片按哈希选择），降低锁竞争；
  - 超时与取消使用 `context`；`time.NewTimer/Stop` 模式替代裸 `time.After`。

---

## 四、示例代码片段

### 1) 安全的 `time.After` 替代
```go
timer := time.NewTimer(d)
defer func() {
    if !timer.Stop() {
        <-timer.C // drain 避免泄漏
    }
}()

select {
case v := <-ch:
    _ = v
case <-timer.C:
    // timeout
}
```

### 2) select 公平性+禁用分支（nil 通道技巧）
```go
var in <-chan T = realIn
var out chan<- T = nil
var pending *T

for {
    select {
    case v := <-in:
        pending = &v
        out = realOut      // 启用 out 分支
    case out <- *pending:
        out = nil          // 发送完禁用 out 分支（防忙轮询）
        pending = nil
    case <-ctx.Done():
        return
    }
}
```

### 3) 分片 channel 降低竞争
```go
type ShardedQ struct {
    shards []chan Job
}
func NewQ(n int, cap int) *ShardedQ {
    q := &ShardedQ{shards: make([]chan Job, n)}
    for i := range q.shards { q.shards[i] = make(chan Job, cap) }
    return q
}
func (q *ShardedQ) Enqueue(j Job) { q.shards[hash(j)%len(q.shards)] <- j }
```

---

