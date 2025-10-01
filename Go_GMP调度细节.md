# Go GMP 调度细节（面试深挖版）

> 面向工程与面试：既讲原理也给“该怎么答”“怎么用”。基于 Go1.21+ 的常识与公开实现细节。

---

## 一、GMP 调度模型（Goroutine Scheduler）

### 1. 角色与数据结构
- **G (goroutine)**：用户级线程，状态机（`_Grunnable/_Gwaiting/_Grunning/_Gsyscall` 等）。
- **M (machine)**：OS 线程，真正执行 G 的载体。数量可动态扩展，默认上限约 10k（受 `runtime/debug.SetMaxThreads` 影响）。
- **P (processor)**：逻辑 CPU，持有可运行队列 **`runq`**、本地定时器、缓存等。**`GOMAXPROCS`** = P 的数量。

```
        +---------+         +---------+
        |   P0    |  ...    |   Pn    |     每个 P 有本地 runq（圆环队列，默认容量 256）
        +----+----+         +----+----+
             |                    |
            M?                   M?
             |                    |
            执行G                 执行G
```

- **本地 runq**：每个 P 独立，减少锁竞争；**work-stealing** 时从其他 P 窃取 **一半** 任务。
- **全局 runq**：系统共享的备用队列，避免局部饥饿；P 会周期性从全局队列拉取 G。
- **netpoller**：epoll/kqueue/IOCP 事件循环，唤醒因 I/O 阻塞的 G（不占用 M）。
- **timers**：定时器（`time.Timer/Ticker`）维护在 **每个 P 的小根堆**（新版本将全局压力下放到 per-P）。

### 2. 调度循环（简版流程）
1. **M 绑定 P**（`acquirep`），在 P 的 `runq` 中取 G 执行；若为空，则：
   - 从 **全局 runq** 拉 G；若仍为空，尝试 **work-stealing**。
   - 如都没有，进入 **自旋/休眠**。
2. **G 发生阻塞**：
   - **系统调用阻塞**（`syscall`）：M 进入 `_Gsyscall`，**释放 P** 给其他 M 使用；syscall 返回后尝试抢回 P，否则把 G 放回可运行队列。
   - **网络 I/O 阻塞**：G 挂到 **netpoller**，M/P 不被占用；I/O 就绪由 poller 唤醒成 runnable。
3. **抢占**：为避免长时间占用 CPU，调度器进行 **协作+异步抢占**：
   - Go1.14 起支持 **异步抢占**（向线程发信号，在安全点抢占），避免“长循环/for 真空期”。
4. **定时器**：P 维护最近到期时间，调度器在无任务时休眠到最近定时器到期或 poller 事件到来。

### 3. 关键细节与调优点
- **自旋 M**：最多保留 `min(GOMAXPROCS, runnableG)` 个自旋线程以降低唤醒延迟，空闲过久会休眠。
- **局部性**：尽量让生产与消费在**同一 P**，减少抢占与窃取开销（例如 worker 池用固定数量 goroutine）。
- **`GOMAXPROCS`**：CPU 密集型任务可设为物理核数；I/O 密集型不要盲目调太大（上下文切换开销）。
- **syscall 与 cgo**：长时间调用会释放 P；可用 **`runtime.LockOSThread`** 固定线程（例如与 GUI/驱动绑定），但会降低并发度。
- **抢占安全点**：在函数调用边界、循环检查点、栈增长等位置；紧凑内联函数过多可能减少安全点。

### 4. 常见面试问答
- **Q：goroutine 是如何被调度的？**  
  **A**：G 通过 P 的本地 runq 被 M 执行；取尽则从全局队列或其他 P 窃取；I/O 阻塞用 netpoller；长占用通过异步抢占打断。
- **Q：syscall 会阻塞整个线程吗？**  
  **A**：会阻塞调用它的 **M**，但 **P 会被释放** 给其他 M，整体并发不受影响；返回后 G 继续执行。

---

## 二、答题模板（30 秒版）

**GMP**：
> “调度器用 **G/M/P**：P 持有本地 runq 与定时器，M 绑定 P 执行 G；取尽则从全局队列或其他 P **窃取一半**任务；阻塞 syscall 释放 P，I/O 由 **netpoller** 挂起与唤醒；Go1.14 起有 **异步抢占**避免长循环霸占 CPU；无任务时按最近定时器休眠，降低延迟与能耗。”

---

## 三、实践清单（落地建议）

- **GMP**：
  - 绑定固定大小的 worker 池，减少任务在 P 间迁移；
  - 避免长时间 CPU 忙循环（会触发抢占），I/O 必须带超时与 ctx；
  - cgo/syscall 前后注意释放/恢复 P 的影响，必要时隔离到专用 worker。

---

## 四、延伸阅读（关键词）
- work-stealing、runq 大小（每 P 默认 256）、netpoll 集成、async preemption、stack growth、sudog、select fairness、per-P timers、`GOMAXPROCS`、`LockOSThread`。

