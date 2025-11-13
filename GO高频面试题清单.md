# Go 高频面试题清单（附要点速记）

> 覆盖语法、并发、内存、网络、工程化、排错以及与 IM 场景相关的高频题。可作复习清单或面试答题提纲。

---

## 1) 语言基础
- **值 vs 引用类型**：值（`int/float/bool/array/struct`）；引用（`slice/map/chan/func`）。引用类型持有指针与元信息。
- **数组 vs 切片**：切片三元组 `ptr,len,cap`；`append` 可能扩容复制；小 cap 近似翻倍，后期增长趋缓。
- **`make` vs `new`**：`new(T)` 返回 *T 零值指针；`make` 仅用于 `slice/map/chan`，返回初始化后的值。
- **字符串与切片互转**：`[]byte(s)` 会复制；注意只读性与生命周期。
- **`defer`**：注册时实参求值；返回前 LIFO 执行；过多有开销（Go1.20+优化）。
- **接口与 nil 陷阱**：接口底层 `(type,data)`；动态类型不为 nil ⇒ 接口不为 nil。

---

## 2) 并发与内存模型
- **GMP 调度**：G=goroutine, M=OS 线程, P=处理器；`GOMAXPROCS` 控制 P 数量。
- **happens-before**：`send→recv`、`Unlock→Lock`、`Done→Wait`、`atomic` 建立顺序。
- **channel 关闭**：只由发送方关闭且仅一次；关闭后接收得到零值且 `ok=false`；向已关 chan 发送会 panic。
- **`select`**：慎用 `default`（可能忙轮询）；超时用 `context` 或 `<-time.After(d)`。
- **`WaitGroup`**：`Add` 在 `Wait` 前；`Done` 与 `Add` 匹配；禁止复制。
- **`atomic` vs `Mutex`**：原子适合简单读写；复杂临界区用锁。
- **goroutine 泄漏**：I/O/chan 阻塞且无取消；用 `context`、超时与关闭通道治理。
- **Ticker/Timer**：记得 `Stop()`；读尽通道避免泄漏。

---

## 3) 容器与数据结构
- **map 并发安全**：默认不安全；并发写会 panic；需要锁或 `sync.Map`；迭代顺序随机。
- **切片共享底层**：子切片 `append` 可能篡改原切片；复制隔离：`append([]T(nil), s...)`。

---

## 4) 错误处理与泛型
- **错误最佳实践**：返回 `error`；`%w` 包装，`errors.Is/As` 判断；避免用 `panic` 作为控制流。
- **泛型**：类型参数、约束（`~`、`comparable`）、对性能与逃逸的影响；API 设计注意约束边界。

---

## 5) 逃逸分析与性能
- **常见逃逸**：返回局部地址、接口/闭包捕获、`fmt` 反射、切片扩容。
- **定位**：`go build -gcflags=-m`、`pprof`、`trace`。
- **`sync.Pool`**：短生命周期热点对象复用；池可能被 GC 清空，不保证取回。
- **基准**：`go test -bench=. -benchmem`；注意基准污染与优化屏蔽。

---

## 6) Context 与超时
- **统一传递 `context`**：所有 I/O 路径携带 `ctx`；`WithTimeout/Deadline` 控时。
- **取消模式**：`select { case <-ctx.Done(): ... }` 释放资源；防 goroutine 泄漏。

---

## 7) I/O、网络与 RPC
- **`net/http`**：复用 `http.Client`；Transport 连接池与超时（`IdleConnTimeout/MaxIdleConnsPerHost`）。
- **gRPC**：拦截器、流式 RPC、Keepalive、重试与负载均衡；压缩与 message 限制。
- **零拷贝**：`io.Copy`/`sendfile`；`bufio` 缓冲；合并写减少 syscalls。

---

## 8) 构建与工程化
- **Modules/工作区**：`go.work`；`replace` 用于本地开发；语义化版本。
- **Build Tags**：`//go:build linux && amd64`；按平台编译不同实现。
- **测试与竞态**：`t.Parallel()`、`-race`；`golang/mock`、`testify`。
- **优雅退出**：捕获 `SIGINT/SIGTERM`；`server.Shutdown(ctx)`；Drain 连接与队列。

---

## 9) GC 与内存
- **GC**：三色标记-清扫；通过减少临时对象、复用缓冲降压；`GODEBUG=gctrace=1` 观察。
- **大对象/Finalizer**：避免依赖 finalizer；大对象易碎片与驻留。

---

## 10) 分布式/IM 关联高频
- **消息不丢**：至少一次 + 幂等；客户端带 `clientMsgId`；服务端先写多副本日志(Kafka `acks=all`/JetStream/WAL)再 ACK；`(convId,msgId)` 唯一约束；离线盒与位点恢复。
- **消息有序（会话内）**：同会话路由同分区/单 actor；服务端分配 `convSeq`；客户端按序渲染，缺口补拉。
- **多端同步**：以 `convSeq` 为准；登录/前台按 `>lastAckSeq` 拉补。
- **限流与背压**：`x/time/rate`，队列丢尾，连接级流控；防推送风暴。
- **大群/热点**：话题/线程分片、读时扩散、批量推送；号段分配降低热点。

---

## 易错/陷阱快问快答
1. **interface nil 陷阱**  
   `var e error = (*MyErr)(nil); e==nil?` → **false**（动态类型非 nil）。
2. **map 并发写？** → 会 panic；加锁或 `sync.Map`。
3. **slice 共享底层导致数据串改？** → 子切片 `append` 可能覆盖；复制隔离。
4. **谁来关 channel？** → **发送方**且仅一次；接收方只读。
5. **`defer` 捕获循环变量？** → 在循环内保存副本或参数立即绑定。
6. **goroutine 泄漏常见源？** → 无超时/ctx 的 I/O、无人接收的 chan、阻塞 select。

---

## IM 场景答题模板（背诵版）
- **有序**：同会话同分区/单 actor 串行，服务端分配 `convSeq`，持久化成功后下发；客户端按 `convSeq` 渲染，缺口补拉。
- **不丢**：至少一次+幂等。客户端 `clientMsgId` 重试；服务端先写多副本日志再 ACK；唯一约束 `(convId,msgId)`；离线盒与位点恢复。
- **重连续传**：带 `(convId,lastAckSeq)` 补拉；恢复未 ACK 的发送（沿用同 `clientMsgId`）。
- **限流/背压**：令牌桶+队列上限；超限拒绝/降级；指标与报警。

---

## 可延伸的深入问题（准备好 1–2 分钟长答）
- Kafka/NATS JetStream 在 IM 场景的选型与参数（`acks=all`、ISR、重放、位点管理）。
- Actor 模式的落地：路由表、一致性哈希、单 writer 的批量提交与零拷贝。
- Redis `INCR` 赋号的热点与号段分配（segment）方案。
- gRPC 长连的 Keepalive、断网重试与服务端流背压。
- `pprof`/`trace` 实战定位：CPU/阻塞/内存与 GC 压力。

---

> 想要带答案的 **“问答卡片版”** 或 **可打印 PDF**，可以告诉我需要的深度与方向（偏基础 / 偏并发 / 偏分布式 / 偏 IM）。
