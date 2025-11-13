# Go Channel 读写注意事项（实战与面试版）

---

## 1) 基本语义要点
- **无缓冲通道**：发送与接收必须“同步会合”，否则阻塞。
- **有缓冲通道**：缓冲满时发送阻塞、缓冲空时接收阻塞。
- **内存模型**：`send` **happens-before** 被 `recv` 读取；`close` **happens-before** 接收方读到 `ok=false`。

---

## 2) 关闭（close）规则（高频坑）
- **谁来关？** **生产者/发送方**负责关闭；接收方只读不关。
- **只关一次**：重复 `close(ch)` 会 panic；向已关闭通道发送也会 panic；从已关闭通道接收会**立即返回零值且 `ok=false`**（缓冲读尽后）。
- **`range ch`**：通道关闭且数据耗尽时退出。
  ```go
  for v := range ch { use(v) } // 正确：发送方关，接收方range
  ```

---

## 3) 非阻塞/超时/取消模式
- **非阻塞发送/接收**（避免忙等）
  ```go
  select {
  case ch <- v:
  default:
    // 满/未就绪：丢弃、降级或计数
  }

  select {
  case v := <-ch:
    _ = v
  default:
    // 暂无数据
  }
  ```
- **超时**（推荐 `NewTimer` 替代裸 `time.After`，避免泄漏）
  ```go
  timer := time.NewTimer(d)
  defer func() { if !timer.Stop() { <-timer.C } }()
  select {
  case v := <-ch:
  case <-timer.C:
  }
  ```
- **取消推荐用 `context`**
  ```go
  select {
  case v := <-ch:
  case <-ctx.Done():
    return
  }
  ```

---

## 4) nil 通道技巧（动态启用/禁用分支）
- 对 `nil` 的发送/接收会永久阻塞，可用于“**禁用某个 select 分支**”。
  ```go
  var out chan<- T // = nil，分支禁用
  var pending *T
  for {
    select {
    case v := <-in:
      pending = &v; out = realOut      // 启用发送
    case out <- *pending:
      out = nil; pending = nil         // 发送后禁用
    case <-ctx.Done():
      return
    }
  }
  ```

---

## 5) 泄漏与清理
- **goroutine 泄漏**：一侧退出而另一侧仍阻塞。→ 统一传递 `ctx`；关闭上游；保证消费者存在。
- **定时器泄漏**：`time.After` 在未被选中时会残留定时器。→ `NewTimer` + `Stop()` + **drain**。
- **退出前 drain**（避免丢消息）
  ```go
  close(ch)                  // 发送方关闭
  for v := range ch { use(v) } // 接收方读尽
  ```

---

## 6) 容量与性能
- **大对象复制**：`chan T` 传值会拷贝，`T` 大时改为 `*T` 或索引。
- **容量选择**：按**生产-消费速率差**与峰值预估；过小易抖动，过大占内存且增加尾延迟。
- **高并发竞争**：共享一个热 `chan` 会锁竞争严重 → 用**分片 channel** 或 **每 worker 私有队列**聚合。
  ```go
  type Q struct{ shards []chan Job }
  // hash(job)%len(shards) 选择分片
  ```

---

## 7) 典型并发模式（读写配方）
- **Worker Pool（固定并发）**
  ```go
  jobs := make(chan Job, N)
  wg := sync.WaitGroup{}
  for i := 0; i < W; i++ {
    wg.Add(1)
    go func() {
      defer wg.Done()
      for {
        select {
        case j, ok := <-jobs:
          if !ok { return }
          handle(j)
        case <-ctx.Done():
          return
        }
      }
    }()
  }
  // 生产
  for _, j := range arr {
    select {
    case jobs <- j:
    case <-ctx.Done():
      break
    }
  }
  close(jobs); wg.Wait()
  ```
- **Fan-in（多源合并）**：每个输入源都要可取消；任何一路阻塞都可能卡住合并器。
- **广播**：一个 `ch` 无法让多路接收者都各拿到一份副本；需要复制/多播层或 `pubsub`。

---

## 8) 正确的结束信号
- **单向结束信号**：用 **`close(done)`** 比 `done<-struct{}{}` 更轻且可多路监听。
  ```go
  done := make(chan struct{})
  // 监听：
  select { case <-done: ... }
  // 发信号：
  close(done)
  ```
- **不要误用 `close(dataCh)` 表示“立即结束”**：`close` 仅表示“不会再有新数据”，未读的数据仍需消费。

---

## 9) 与锁的取舍
- `chan` 语义清晰且内置可见性保证（HB 关系）；极端低延迟/短临界区场景下，**锁 + 环形队列**可能更快。依据：可读性、延迟目标、竞争程度。

---

## 10) 调试与排错
- **`-race`** 查数据竞争；`pprof` 看 goroutine 是否阻塞在 `chan send/recv`。
- 区分“零值”与“通道关闭”：
  ```go
  v, ok := <-ch
  if !ok { /* 通道已关闭且耗尽 */ }
  ```

---

## 面试 30 秒速记
- 发送方关，且只关一次；读关闭通道返回零值+`ok=false`。
- 非阻塞读写用 `select{case ... default:}`；超时/取消用 `select + context`；`NewTimer` 替代 `time.After`。
- `nil` 通道可“禁用分支”；退出前 drain，防泄漏。
- 大对象传指针；热通道分片；容量按峰值与速率差估算。
