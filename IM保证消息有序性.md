# IM 系统如何保证**消息有序性**（会话内强有序）

## 结论（先记住这三点）
- **按会话维度有序**：只保证同一会话（conversation）内的顺序，不做全局有序。
- **服务端分配顺序号**：消息到达服务端后，由服务端统一分配会话序号 **`convSeq`**。
- **客户端按 `convSeq` 展示**：短暂乱序没关系，通过缓冲与补拉最终一致。

---

## 核心原则
1. **同会话单写通道**：同一会话的消息路由到同一个“单线程/单分区/单 actor”处理，天然顺序。
2. **服务端赋号 + 持久化后可见**：`convSeq` 在**持久化成功**的原子步骤中分配；成功后才 ACK/下发。
3. **幂等去重**：客户端必须带 **`clientMsgId`**；服务端对 `(convId, clientMsgId)` 幂等，重复返回**第一次的 `convSeq`**。

---

## 服务端设计（保证同会话单调递增）

### 路由与串行化
- **路由键**：`conversationID`
- **实现方式**（三选一）
  - **Actor/Single-Writer**：`convID → 某个 actor`，该 actor 单线程处理该会话的所有消息。
  - **Kafka 分区**：`partition = hash(convID) % N`，同会话同分区，分区内有序。
  - **Redis 计数器**：`INCR conv:{id}:seq` 获取序号（需与写入日志/库的事务一致）。

### 赋号与持久化（伪码）
```go
// 同会话内串行
a.seq++                 // 服务端分配 convSeq（单调递增，允许跳号）
m.ConvSeq = a.seq
off := wal.Append(m)    // 先写日志/多副本(如 Kafka/NATS/WAL)
store.Save(m, off)      // 再写业务存储：主键 (convID, convSeq)
idem.Put(convID,msgID, m.ConvSeq) // 幂等映射
fanout.Push(m)          // 推给在线/写离线盒
ack(sender, m.ConvSeq)  // 返回 ACK（带 convSeq、offset）
```
> **要点**：赋号与持久化**同一写路径**内完成；崩溃可重放，顺序不乱。

---

## 客户端处理（乱序缓冲 + 缺口补拉）

- **只信 `convSeq` 排序**，不要用时间排序。
- **状态机**：
  - 维护 `nextSeqExpected` 与临时缓冲 `buffer`。
  - 收到 `seq == next`：立即展示并推进 `next`；同时尝试从 `buffer` 连续释放。
  - 收到 `seq > next`：放入 `buffer`，若超时仍有缺口，则 **拉取补齐**（`pullMissing(next, seq-1)`）。

```go
next := lastSeen + 1
buf := map[uint64]Msg{}

onRecv(m):
  if m.ConvSeq == next {
    display(m); next++
    for buf[next] != nil { display(buf[next]); delete(buf, next); next++ }
  } else if m.ConvSeq > next {
    buf[m.ConvSeq] = m
    // 超时仍缺口 => pullMissing(next, m.ConvSeq-1)
  } // m.ConvSeq < next => 重复，丢弃
```

---

## 幂等与重试（必要条件）
- **客户端生成 `clientMsgId`**（UUID/ULID/Base64 128-bit 随机）；超时重投**必须复用同一个 `clientMsgId`**。
- **服务端唯一约束**：
```sql
ALTER TABLE im_message ADD UNIQUE KEY uk_conv_msg (conv_id, client_msg_id);
```
  重复消息直接返回第一次的 `convSeq`，**不再写新记录**。

---

## 方案对比（如何落地）

| 方案 | 路由/有序性 | 赋号方式 | 优点 | 注意事项 |
|---|---|---|---|---|
| **Actor/Single-Writer** | 每会话一个 actor 串行 | 内存自增或号段 | 实现直观、延迟低 | 热点会话需扩容单writer能力 |
| **Kafka 分区** | `hash(convID)` 同分区 | 消费侧顺序消费后赋号 | 天然有序、可重放 | 热点分区；消费者位点管理 |
| **Redis INCR** | 任意路由 | `INCR conv:{id}:seq` | 简单易用 | 必须与写入一致性处理，防跳号乱序 |

> **允许跳号**：因故失败或清理可能出现空洞，但排序不受影响；客户端按 `convSeq` 展示即可。

---

## 常见坑 & 规避
- 用**客户端时间**排序 → ❌（时钟不一致）。  
- **客户端不带 msgId** → ❌（重试会写多条，顺序混乱）。  
- 多设备并发发送 → ✅（服务端统一赋 `convSeq`，客户端按序渲染即可）。  
- 大群热点 → 对群内做**子话题分片**或提升单 writer 吞吐（批量、零拷贝、锁分离）。

---

## 面试速记（20 秒版本）
> “我们把**会话**作为有序单元：同会话消息路由到同一写通道（Kafka 同分区/服务端单 actor），服务端**统一分配会话序号 `convSeq`**，并在**持久化成功后**再 ACK/下发。客户端只按 `convSeq` 渲染，遇到缺口用**缓冲+补拉**。客户端用 `clientMsgId` 幂等重试，服务端对 `(convId,msgId)` 做唯一约束，重复返回第一次的 `convSeq`。这样能实现**会话内强有序、短暂乱序最终一致**。”
