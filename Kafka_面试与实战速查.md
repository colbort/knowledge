# Kafka 面试与实战速查手册（综合版）

> 覆盖：核心概念（Topic/Partition/Replica）、Producer/Consumer 要点、可靠性语义、存储与副本、EOS、调优与排障、运维与 KRaft、常见陷阱与面试速答。

---

## 目录
1. 核心概念与架构总览
2. Topic / Partition / Replica（深入）
3. Producer（生产者）——分区、批量、幂等等
4. Consumer（消费者/消费者组）——位点、重平衡、语义
5. 端到端可靠性：顺序、不丢、不重（EOS）
6. 存储与副本：日志段、HW/LEO、compaction
7. 性能与调优：生产端/消费端/Broker
8. 安全、多租户与参数模板
9. 运维与 KRaft、重分配与监控
10. 典型陷阱与排障清单
11. 高频问答（面试快问快答）
12. 术语表与背诵清单

---

## 1. 核心概念与架构总览

- **Topic**：消息的逻辑分类。
- **Partition**：并行与顺序的最小单位；分区内严格有序，跨分区无序。
- **Replica**：每分区 1 个 Leader + N 个 Follower；Follower 拉取复制。
- **Broker**：存储和服务节点；**Controller** 负责元数据、选主（Kafka 3.x 可用 **KRaft** 取代 ZK）。
- **Producer / Consumer / Consumer Group**：
  - Producer 选择分区写入，支持批量与压缩。
  - Consumer 按 **组** 消费；同组内一个分区同一时刻只分配给一个消费者。
- **可靠性参数**：`acks`、`min.insync.replicas`、`enable.idempotence`、`transactional.id`。
- **语义**：at-most-once / at-least-once / exactly-once（事务）。

---

## 2. Topic / Partition / Replica（深入）

### 2.1 概念与作用
- **Topic** 只是逻辑名；真正的并发度由 **Partition** 决定。
- **Partition**：顺序单位；把同一业务键（如订单ID）路由到同一分区即可保证该键的顺序。
- **Replica**：提高可用性与持久性。只有 **Leader** 对外读写；**ISR**（in-sync replicas）是与 Leader 同步落后的可接受集合。

### 2.2 HW / LEO
- **LEO（Log End Offset）**：本地日志末尾位置。
- **HW（High Watermark）**：对外可见的最高偏移（需被 ISR 多数确认）。消费者默认只能读到 ≤ HW 的记录。

### 2.3 分区数量怎么定？
- 估算：`分区数 ≈ 峰值吞吐 / 单分区吞吐 × 冗余(1.2~2)`。
- 过多分区会增加 **FD/内存/元数据** 压力与控制面开销；可后续 **重分配** 平衡。

### 2.4 保留与压缩
- **删除**：`log.retention.hours|bytes` 到阈值删除旧段。
- **压实（compaction）**：按 key 保留最新记录，旧版本异步清理；适合状态流/CDC。`cleanup.policy=compact` 或 `compact,delete`。

### 2.5 面试要点
- **顺序**：分区内天然有序；同 key → 同分区。
- **不丢**：`acks=all` + `min.insync.replicas≥2` + 关闭 `unclean.leader.election` + 多 AZ。
- **为什么用 ISR**：限定可选主的健康集合，降低回滚风险。

---

## 3. Producer（生产者）——分区、批量、幂等等

### 3.1 分区选择
- 有 **key**：默认按 key 哈希到固定分区（保证该 key 的顺序）。
- 无 key：**黏性分区器**（sticky）把短时消息黏到同一分区，提高批量命中率。

### 3.2 批量与压缩
- `batch.size`（批大小）与 `linger.ms`（等待时间）共同决定批量；
- `compression.type=lz4|zstd|snappy` 节省网络/磁盘、提高吞吐。

### 3.3 幂等与事务
- **幂等**：`enable.idempotence=true` + `acks=all` + `retries>0`，限制乱序（`max.in.flight.requests.per.connection≤5`，新版本放宽）；避免重复写。
- **事务（EOS）**：设置 `transactional.id`，在一个事务内 **写消息 + 提交位点**；下游 `read_committed` 只读已提交。

### 3.4 关键参数（建议起点）
```properties
acks=all
enable.idempotence=true
retries=2147483647
linger.ms=5~20
batch.size=64KB~256KB
compression.type=lz4|zstd
max.in.flight.requests.per.connection=1~5
request.timeout.ms=30000
delivery.timeout.ms=120000
```

### 3.5 常见问题（面试）
- acks=all 是否绝对安全？→ 还需 `min.insync.replicas` 保证副本数。
- 为什么会乱序？→ 重试 + in-flight 并发导致；幂等 + 限并发可规避。
- 粘性分区器价值？→ 提升批量与吞吐，降 syscall/磁盘碎片。
- EOS 边界？→ Kafka 内端到端；跨外部系统不保证，需要幂等/两阶段等。

---

## 4. Consumer（消费者/消费者组）——位点、重平衡、语义

### 4.1 消费者组与位点
- 同组**共享分区**：分区与组内消费者**一对一**。
- **offset** 存于 `__consumer_offsets`；提交方式：同步（严格）/异步（吞吐）/自动（不推荐生产）。

### 4.2 重平衡（Rebalance）
- **触发**：成员变更、分区变更、会话/心跳超时。
- **策略**：`range` / `roundrobin` / `sticky` / **`cooperative-sticky`（增量）**。
- **超时**：`session.timeout.ms`、`heartbeat.interval.ms`、`max.poll.interval.ms`（处理过慢会被踢组）。

### 4.3 消费语义
- **At-most-once**：先提交再处理（可能丢，不重复）。
- **At-least-once**：处理后提交（不丢，可能重复）——主流方案，配合幂等下游。
- **Exactly-once**：事务性写回 + `read_committed` + 原子提交位点（Kafka Streams/事务 API）。

### 4.4 参数建议（起点）
```properties
group.id=your-app
enable.auto.commit=false
auto.offset.reset=latest|earliest
max.poll.records=500~2000
max.poll.interval.ms=300000
fetch.min.bytes=1KB~64KB
fetch.max.wait.ms=50~200
max.partition.fetch.bytes=1MB~8MB
isolation.level=read_committed   # 若使用事务
partition.assignment.strategy=org.apache.kafka.clients.consumer.CooperativeStickyAssignor
```

### 4.5 背压与并行
- 放大 `fetch.min.bytes` / `max.poll.records` 批量，配合线程池处理；
- 处理不过来 → 降并发/限流/扩实例；避免超 `max.poll.interval.ms` 被踢。

---

## 5. 端到端可靠性：顺序、不丢、不重（EOS）

- **顺序**：以分区为单位；同 key 路由到同一分区；消费端单线程/有序队列处理单分区。
- **不丢**：生产端 `acks=all` + `min.insync.replicas≥2` + 重试；Broker 多副本且关闭 `unclean.leader.election`；消费端**处理后提交**。
- **不重**：生产端幂等；消费端幂等写下游或使用事务把“写出+提交位点”原子化。

---

## 6. 存储与副本：日志段、HW/LEO、Compaction

- 分区由**若干段文件（segment）**组成；每段有**索引**与**时间索引**。写入是**追加**。
- 复制：Follower **从 Leader 拉取**；**HW** 决定可见进度；**LEO** 为本地进度。
- `unclean.leader.election.enable=false`（默认）避免从非 ISR 选主导致回滚。
- **Log Compaction**：按 key 保留最新版本；删除通过 **tombstone** 记录。

---

## 7. 性能与调优

### 7.1 生产端
- 批量：`linger.ms`、`batch.size`；压缩：`lz4/zstd`。
- 顺序/乱序：控制 `max.in.flight`，开启幂等。
- 可靠性：`acks=all` + `min.insync.replicas`。

### 7.2 消费端
- 批量拉取：`fetch.min.bytes`、`fetch.max.wait.ms`、`max.partition.fetch.bytes`。
- 处理节奏：`max.poll.records`；防 `max.poll.interval.ms` 超时。

### 7.3 Broker
- 分区规划：适量即可；避免过多分区。
- 磁盘/网络：NVMe、足带宽；利用 OS PageCache + `sendfile` 零拷贝。
- JVM：G1/ZGC；堆不宜过大，PageCache 更重要。

---

## 8. 安全、多租户与参数模板

### 8.1 安全
- **SASL/SSL**：SCRAM/OAUTHBEARER/TLS；生产环境开启加密与认证。
- **ACL**：按 Topic/Group/Cluster 控制，最小权限。

### 8.2 配置模板（示意）
**Producer**
```properties
acks=all
enable.idempotence=true
retries=2147483647
linger.ms=5~20
batch.size=64KB~256KB
compression.type=lz4|zstd
max.in.flight.requests.per.connection=1~5
```
**Consumer**
```properties
group.id=app-x
enable.auto.commit=false
auto.offset.reset=latest
max.poll.records=1000
max.poll.interval.ms=300000
fetch.min.bytes=16KB
fetch.max.wait.ms=100
partition.assignment.strategy=org.apache.kafka.clients.consumer.CooperativeStickyAssignor
```
**Broker 片段**
```properties
num.partitions=3
default.replication.factor=3
min.insync.replicas=2
unclean.leader.election.enable=false
log.retention.hours=168
log.cleanup.policy=delete|compact
```

---

## 9. 运维与 KRaft、重分配与监控

- **KRaft**：无 ZK 的内置 Raft 元数据模式（3.x 成熟）；部署更简化。
- **分区重分配**：`kafka-reassign-partitions.sh` 或 **Cruise Control** 自动均衡。
- **扩容**：新增 Broker 后迁移副本均衡 I/O。
- **监控**：
  - 生产：`record-send-rate`、`request-latency`、`retries`；
  - 消费：**consumer lag**、`records-lag-max`、rebalance 次数；
  - Broker：`UnderReplicatedPartitions`、`ActiveControllerCount`、`RequestHandlerAvgIdlePercent`、磁盘与网络利用率。

---

## 10. 典型陷阱与排障清单

- 分区不足或热点 key → 吞吐不达标/倾斜。
- `acks=1` 在故障下丢数据；`unclean.leader.election` 可能回滚。
- **大消息**（>1MB）需要同步放大大小限制；推荐拆分或外部存储引用。
- 消费者处理过慢 → 超 `max.poll.interval` 被踢组，频繁 rebalance。
- EOS 误用：跨系统无法自动保证恰好一次，需幂等/外部协调。

**排障 Checklist**
- 写入失败/超时：看 `request-latency`、`retries`、网络/磁盘、ISR 状态。
- 消费积压：增批量、并行；检查下游瓶颈；必要时扩分区/实例。
- 顺序乱：按 key 分区；控制 `max.in.flight`；开启幂等。
- 保留异常：核对 `log.retention.*` 与 `cleanup.policy`；compaction 是异步的。

---

## 11. 高频问答（面试快问快答）

- **分区内顺序怎么保证？** 分区内天然有序；同 key 路由到同一分区；消费端单线程/有序队列处理。  
- **如何做到不丢不重？** 不丢：`acks=all` + `min.insync.replicas` + 处理后提交；不重：幂等生产/幂等下游或事务。  
- **为什么会 rebalance？** 成员变更、会话超时、分区变化；用 cooperative-sticky，控制 `max.poll.interval`。  
- **Log Compaction 原理？** 按 key 保留最新，旧版本异步清理；删除用 tombstone。  
- **acks=all 就一定安全吗？** 需要 `min.insync.replicas≥2`，并关闭 unclean 选主。

---

## 12. 术语表与背诵清单

- 术语：ISR、HW、LEO、Compaction、Sticky Partitioner、Cooperative Rebalance、Idempotent Producer、EOS、KRaft、Rack Awareness。

**30 秒背诵**：  
> “分区内有序；同 key 同分区。`acks=all` + `min.insync.replicas` 不丢；幂等或事务不重。消费者组一对一，cooperative-sticky 减重平衡。批量+压缩提吞吐；监控 Lag/URP/时延/磁盘网络。KRaft 简化元数据，Cruise Control 做均衡。”
