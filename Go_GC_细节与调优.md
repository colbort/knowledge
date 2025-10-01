# Go 垃圾回收（GC）细节与调优实战

> 面向工程与面试：讲明 **原理 → 阶段 → 写屏障 → 分配器 → 调优**。以 Go 1.19–1.22 行为为主（GOMEMLIMIT/软内存上限、per-P timers 等特性）。

---

## 1. 总览：标记-清扫、并发、非移动（heap）
- **算法**：并发**三色标记（tri-color） + 标记-清扫（mark-sweep）**；**非移动**堆（heap 不搬迁），**栈可移动**（缩放/拆分时复制）。
- **STW 阶段很短**：仅在 **Mark Enter（扫尾/根快照）** 与 **Mark Termination（回收结束）** 做微小 STW；标记与清扫基本**并发**。
- **写屏障（write barrier）**：Dijkstra 插入写屏障，保证并发标记期的可达性不漏标。
- **目标**：将 GC 对 CPU 的影响（pacer 目标）控制在较低比例，典型 **~25%** 的上限预算（动态变化）。

---

## 2. 堆增长目标（soft heap goal）
- **live = 上一轮标记后存活字节**。  
- **GOGC**：增量百分比（默认 100）。  
- **下轮目标**：
  ```text
  heapGoal ≈ live * (1 + GOGC/100)
  ```
- **GOMEMLIMIT（1.19+）**：**软内存上限**（进程维度），当 heap + stacks + globals 接近该上限时，**pacer 提前收紧**，加速 GC，抑制堆继续增长。
  - 典型设置：`GOMEMLIMIT=2GiB` 或运行时 `debug.SetMemoryLimit()`。
  - 相比“内存压舱石（ballast）”更优雅。

---

## 3. GC 周期与阶段（简化）
1) **Sweep Termination（STW）**：结束上一轮清扫的收尾；进入新一轮标记。  
2) **Mark**（并发标记）：
   - **根扫描**：G 栈、全局变量、寄存器中的指针。部分 STW。
   - **灰化/黑化**：三色不变式维护（白=未标，灰=已发现待扫描，黑=已扫描）。
   - **写屏障**：记录新建/更新指针，避免遗漏。
   - **Mark Assist（分配方助力）**：分配触发“**按比例扣费**”：申请越多，越要参与推进标记。
3) **Mark Termination（STW）**：确保队列清空、完成最终标记与统计。  
4) **Sweep（并发清扫）**：按 span 清理白对象，返回自由列表。后台清扫线程 + 惰性清扫（分配时顺带）。

> **并发性**：Mark/Sweep 大部分时间与业务 goroutine 并发执行，STW 窗口通常 < 数毫秒级。

---

## 4. 写屏障（Write Barrier）与三色不变式
- **为何需要**：并发标记时，mutator（用户代码）可能改变对象图，导致“已扫描的黑对象指向白对象”。
- **Go 的做法**：**插入写屏障**（Dijkstra），在执行 `*p = q` 这类指针写时：
  ```text
  1) 将被写入的目标对象 q 标为灰（入标记队列）
  2) 记录卡表（card marking）以缩小扫描面（优化）
  ```
- **影响**：写屏障让写指针变慢一点点（汇编插桩），换来并发标记的低停顿。编译器在 GC 标记期开启屏障。

---

## 5. 分配器（mcache/mcentral/mheap）与对象分类
- **arena/span/page**：堆被分为 **arena**（区）、每区由若干 **span** 组成，span 管理某一 **size class** 的对象块（page 为小页）。
- **mcache（每 P 私有）**：线程本地缓存，减少锁争用；优先从 mcache 分配对象；不足时向 mcentral 申请。
- **mcentral（全局每 size class）**：维护空闲 span 列表；枯竭时向 mheap 申请新的 span。
- **mheap（全局）**：管理整个堆的空闲区（bitmap/treap）。
- **小对象**：按 size class（8B~32KB）分配；**大对象（>32KB）** 直接向 mheap 申请整 span。  
- **noscan 对象**：不含指针的对象进入 **noscan 列**，GC 扫描更快（少指针追踪）。  
- **tiny 分配**：极小对象（≤16B 且无指针）走 tiny 空间批量塞入，减少碎片与元数据开销。

---

## 6. 标记栈、根扫描与栈收缩
- **根**：全局指针、寄存器与各 goroutine 的栈。栈上指针通过编译期生成的指针图（stack map）精准扫描。  
- **栈可移动**：goroutine 的栈可伸缩（从 2KB 起按需扩容/收缩），扩缩时复制栈内容并修正指针。  
- **safe point**：在函数调用边界/循环检查处插入抢占点，异步抢占时在安全点暂停 G，保障扫描一致性。

---

## 7. 调优与参数（实战）
### 7.1 什么时候调 `GOGC`
- **降低延迟**（更频繁 GC）：`GOGC` 小一些（如 50），堆更小、GC 更频繁、CPU 更高。  
- **降低 CPU 占用**（更少 GC）：`GOGC` 大一些（如 200~500），堆更大、GC 更少、峰值内存上升。  
- **建议**：先观察，若 **CPU 因 GC 很高** → 增大 `GOGC`；若 **RSS 太大** → 减小 `GOGC` 或设置 `GOMEMLIMIT`。

### 7.2 软内存上限 `GOMEMLIMIT`
- **作用**：在容器/k8s 限额下避免 OOM。设置后，pacer 会更积极回收。  
- 设置方式：环境变量 `GOMEMLIMIT=2GiB` 或运行时 `debug.SetMemoryLimit(2<<30)`。

### 7.3 常用调试开关
- **`GODEBUG=gctrace=1`**：打印每轮 GC 的时间、堆大小、CPU 百分比等。
- **`GODEBUG=gcstoptheworld=1`**：观察 STW 行为（实验/调试）。
- **`GODEBUG=madvdontneed=1`**：收缩堆时更积极地归还内存给 OS（可能影响抖动）。
- **`runtime.ReadMemStats`**：采样获取堆使用/对象数/下一次 GC 目标等。

### 7.4 代码层优化
- **减少分配**：热点路径复用 `bytes.Buffer`/`sync.Pool`；避免 `[]byte → string` 不必要拷贝（可用 `unsafe` 有风险）。
- **按需切片容量**：预估 `make([]T, 0, n)` 减少扩容与复制。  
- **避免 finalizer**：不可预期，延迟释放；倾向于显式 `Close`/`Put`。  
- **选择无指针结构**：使对象进入 noscan（如大块二进制缓冲）。
- **长生存缓存**：注意缓存淘汰策略，避免持有大量对象导致“存活集”过大（进而抬高 heapGoal）。

---

## 8. gctrace 示例解读
示例输出（简化）
```
gc 56 @11.234s 0%: 0.9+3.2+0.2 ms clock, 3.6+0.8/2.1/1.1 ms cpu, 512->300->320 MB, 600 MB goal, 8 P
```
- `gc 56`：第 56 次 GC。  
- `0.9+3.2+0.2 ms`：分别是 **STW mark setup** + **并发标记** + **STW mark termination**。  
- `512->300->320 MB`：开始堆→存活→结束堆（含碎片）。  
- `600 MB goal`：heapGoal。  
- `8 P`：GOMAXPROCS。

---

## 9. 常见问题与答法
- **Q：Go 的 GC 是如何保证低停顿的？**  
  **A**：并发标记-清扫、仅在标记开始/结束有短暂 STW，写屏障与抢占点保证并发安全；pacer 控制 GC 预算。

- **Q：Go 是移动 GC 吗？**  
  **A**：堆**非移动**（指针稳定），**栈可移动**（按需扩缩）。

- **Q：如何控制内存上限/避免 OOM？**  
  **A**：`GOMEMLIMIT` 设软上限，配合监控与 `GOGC` 调整。容器内还要考虑 cgroup 限制。

- **Q：`sync.Pool` 一定能降内存吗？**  
  **A**：降低短期分配与 GC 压力，但 GC 时池可能被清空；不保证取回，适合热点、易复用对象。

- **Q：为什么 GC 很频繁/CPU 高？**  
  **A**：存活集大、分配速率高、GOGC 太小、`Sync.Pool` 不当使用、对象含指针导致扫描成本高。

---

## 10. 实战排查路线（Checklist）
1) **打开 gctrace/pprof**：评估 GC 周期、耗时、占比、堆峰值。  
2) **看分配热点**：`pprof -alloc_objects/-alloc_space`，找出高频构造。  
3) **降低分配**：复用缓冲、池化、避免临时对象与接口导致的逃逸。  
4) **压住堆峰值**：优化缓存淘汰；必要时调小 `GOGC` 或设 `GOMEMLIMIT`。  
5) **结构优化**：无指针（noscan）、紧凑结构、减少跨对象指针。  
6) **灰度验证**：关注 tail latency（P99）与吞吐变化。

---

## 11. 代码片段

### 11.1 设置软内存上限（GOMEMLIMIT）
```go
import "runtime/debug"

func init() {
    // 2GiB 软上限，视业务而定；可动态调整
    debug.SetMemoryLimit(2 << 30)
}
```

### 11.2 gctrace 与 memstats
```go
import (
    "fmt"
    "runtime"
)

func dumpMemStats() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    fmt.Printf("HeapAlloc=%dMB HeapSys=%dMB NumGC=%d NextGC=%dMB\n",
        m.HeapAlloc>>20, m.HeapSys>>20, m.NumGC, m.NextGC>>20)
}
```

### 11.3 使用 sync.Pool（注意生命周期）
```go
var bufPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}

func useBuf() {
    b := bufPool.Get().(*bytes.Buffer)
    b.Reset()
    defer bufPool.Put(b) // 归还以便复用；GC 时可能被清空
    // ...
}
```

---

## 12. 面试 30 秒版
> “Go 的 GC 是**并发三色标记-清扫**，堆**不移动**，只在标记开始/结束有极短 **STW**。通过 **写屏障** 维护三色不变式，**Mark Assist** 让分配方参与推进，**pacer** 依据 `heapGoal=live*(1+GOGC/100)` 控制频率。1.19 起有 **GOMEMLIMIT** 软上限，配合 `GOGC` 调整 CPU/内存权衡。优化方向是**减少分配、降低存活集、无指针对象、合理池化**，诊断用 `gctrace`/`pprof`/`ReadMemStats`。”

---

### 关键词备忘：
`tri-color`、`write barrier`、`mark assist`、`pacer`、`heapGoal`、`GOGC`、`GOMEMLIMIT`、`mcache/mcentral/mheap`、`noscan`、`tiny alloc`、`STW`、`pprof`、`gctrace`。
