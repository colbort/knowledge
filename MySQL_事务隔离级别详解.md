# MySQL（InnoDB）事务隔离级别详解（面试/实战）

> 覆盖：四级隔离、读现象、InnoDB MVCC/锁、快照读 vs 锁定读、示例脚本、选型与常见坑。

---

## 1. 基础概念回顾
- **ACID**：原子性（Atomicity）、一致性（Consistency）、隔离性（Isolation）、持久性（Durability）。
- 三种典型读异常：
  - **脏读（Dirty Read）**：读到未提交数据。
  - **不可重复读（Non-Repeatable Read）**：同一事务内两次读同一行，看到不同版本（他人提交了修改）。
  - **幻读（Phantom Read）**：同一事务内同条件多次查询，**行集合数量**发生变化（他人插入/删除导致“幽灵行”出现/消失）。

---

## 2. 四个隔离级别（SQL 标准）
| 隔离级别 | 脏读 | 不可重复读 | 幻读 | MySQL InnoDB 默认 |
|---|---|---|---|---|
| READ UNCOMMITTED | 可能 | 可能 | 可能 | 否 |
| READ COMMITTED   | 不会 | 可能 | 可能 | 否 |
| REPEATABLE READ  | 不会 | 不会 | **标准下可能**；InnoDB 通过 **Next-Key 锁** 抑制锁定读下的幻读 | ✅（默认） |
| SERIALIZABLE     | 不会 | 不会 | 不会 | 否 |

> **要点**：InnoDB 的 **REPEATABLE READ（RR）** + **Next-Key 锁** 能防住“锁定读”场景下的幻读；“快照读”在 RR 下读的是同一“读视图”（Read View），从读者角度自然不再出现“变多/变少”的结果。

---

## 3. InnoDB 如何实现隔离：MVCC + 锁

### 3.1 MVCC（多版本并发控制）与 Read View
- **快照读（Consistent Read）**：普通 `SELECT`（不带 `FOR UPDATE/LOCK IN SHARE MODE`），读取历史版本，**不加行锁**。
- **Read View（读视图）**：
  - **READ COMMITTED**：**每条语句**创建新的 Read View → 同一事务的两次 `SELECT` 可能看到不同结果（不可重复读）。
  - **REPEATABLE READ**：**第一次快照读**创建并在事务内复用 Read View → 之后的快照读结果保持一致。
- 直观理解：**RC 看“提交时刻的最新世界”，RR 看“开启后固定的那张照片”。**

### 3.2 锁：记录锁 / 间隙锁 / Next-Key 锁
- **记录锁（Record Lock）**：锁具体索引记录。
- **间隙锁（Gap Lock）**：锁索引区间的“空隙”，防止并发插入穿越。
- **Next-Key 锁**：记录锁 + 右侧间隙锁（半开半闭区间），用于**锁定读**时抑制幻读。
- **意向锁（IS/IX）**：表级元锁，用于快速判定是否存在行级锁冲突。
- **插入意向锁**：并发插入时的协调锁。

> **无合适索引**时，锁范围可能被放大（甚至近似全表范围）；务必关注执行计划。

---

## 4. 各隔离级别下的表现（InnoDB 语义）

### 4.1 READ UNCOMMITTED（几乎不用）
- 允许脏读；不可重复读/幻读皆可能。

### 4.2 READ COMMITTED（RC）
- 每条语句单独 Read View → 避免脏读，但可能**不可重复读/幻读**。
- **锁定读**（`FOR UPDATE/SHARE`）时，仍会对命中的记录与间隙加锁（取决于索引），可规避范围插入穿透。

### 4.3 REPEATABLE READ（RR，默认）
- 事务内快照读固定视图 → 避免不可重复读；从读者视角“幻读”也不再出现。
- **锁定读 + Next-Key 锁** → 抑制并发插入导致的幻读。

### 4.4 SERIALIZABLE
- 所有 `SELECT` 隐式升级为共享锁（读读也互斥），吞吐下降明显。

---

## 5. 快照读 vs 锁定读（一定要分清）
- **快照读**：`SELECT ...`（不带锁）→ 不加行锁、读历史版本，高并发。
- **锁定读**：`SELECT ... FOR UPDATE/LOCK IN SHARE MODE` → 对命中范围加 **记录锁 + 间隙锁**（依赖索引），用于“读后改”“范围一致性”。
- **写操作**（`UPDATE/DELETE/INSERT`）：本质都是“锁定读 + 写”。

> **实践建议**：需要“读后改/防插入穿透”的逻辑，使用 **锁定读 + 正确索引**。

---

## 6. 示例脚本：RC vs RR 的观感差异

### 6.1 准备数据
```sql
CREATE TABLE acct (
  id BIGINT PRIMARY KEY,
  balance INT,
  KEY idx_balance (balance)
) ENGINE=InnoDB;

INSERT INTO acct VALUES (1, 100), (2, 200), (3, 300);
```

### 6.2 READ COMMITTED（RC）：不可重复读与“集合变化”
会话 A：
```sql
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
START TRANSACTION;
SELECT balance FROM acct WHERE id=1;        -- 读到 100（语句级视图）
-- 暂停，等待会话 B 修改并提交
SELECT balance FROM acct WHERE id=1;        -- 读到 150（看到 B 的提交）
COMMIT;
```

会话 B：
```sql
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
UPDATE acct SET balance=150 WHERE id=1;
COMMIT;
```

### 6.3 REPEATABLE READ（RR）：事务内读一致
会话 A：
```sql
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
START TRANSACTION;
SELECT balance FROM acct WHERE id=1;        -- 100（创建读视图）
-- 等 B 修改提交
SELECT balance FROM acct WHERE id=1;        -- 仍然 100（快照读不变）
COMMIT;
-- 事务结束后新的查询将看到最新值 150
```

### 6.4 RR 下的“锁定读防幻读”
会话 A：
```sql
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
START TRANSACTION;
-- 锁定 balance 在 [100, 200] 的记录及其间隙（依赖 idx_balance 命中）
SELECT * FROM acct WHERE balance BETWEEN 100 AND 200 FOR UPDATE;
-- ... 业务处理 ...
-- 在提交前，会话 B 在该范围的插入会被阻塞（避免“幻读”插入穿透）
COMMIT;
```

会话 B：
```sql
INSERT INTO acct(id,balance) VALUES(10, 150);  -- 将阻塞至 A 提交（Next-Key 锁生效）
```

> **注意**：上例锁定范围依赖 `idx_balance` 被使用。若查询条件不走索引，InnoDB 可能扩大锁范围（影响并发）。

---

## 7. 选型与实战建议
- **默认用 RR（MySQL 习惯）**：读一致性强，结合“锁定读+索引”可防幻读；适合绝大多数 OLTP。
- **需要“最新提交可见”语义**：用 RC（与 Oracle/PG 语义一致）；需要范围一致性时也用“锁定读”。
- **强一致串行**：SERIALIZABLE（慎用，吞吐差）。
- **避免** RU（除极端只读且容忍脏读）。
- **务必短事务**：长事务会拖累 **purge**，导致 undo 与历史版本膨胀。

---

## 8. 常见问答（面试高频）
- **Q：InnoDB 的 RR 会不会有幻读？**  
  **A**：快照读在 RR 下读的是同一视图（从读者角度不会“凭空多行”）；锁定读使用 **Next-Key 锁** 抑制并发插入穿透（写者角度避免幻读）。通常认为 **不会**，除非关闭 gap 锁或无索引扫描等特殊情况。

- **Q：为什么我在 RC 下两次同条件查询结果不同？**  
  **A**：RC 每条语句新建 Read View，第二次查询自然能看到别人已提交的修改/插入。

- **Q：如何查看/设置隔离级别？**  
  ```sql
  SELECT @@transaction_isolation;  -- 或 @@tx_isolation(老版)
  SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
  SET GLOBAL  TRANSACTION ISOLATION LEVEL REPEATABLE READ; -- 只影响新连接
  ```

- **Q：如何避免“范围插入穿透”？**  
  **A**：使用 **锁定读（FOR UPDATE/SHARE）+ 正确索引条件**，触发 Next-Key 锁封锁区间。

- **Q：间隙锁什么时候生效？**  
  **A**：在 **RR/RC** 隔离级别下的**锁定读/更新/删除**操作中，对命中范围加 Gap/Next-Key 锁；**普通快照读**不加 gap 锁。

---

## 9. 观察与排障
- 观察锁等待：
  ```sql
  SELECT * FROM performance_schema.data_locks;
  SELECT * FROM sys.innodb_lock_waits\G
  SHOW ENGINE INNODB STATUS\G
  ```
- 观察执行计划：`EXPLAIN [ANALYZE]`，确认是否命中索引、锁粒度是否合理。
- 观察历史版本与 purge：`INNODB_METRICS`、`information_schema.innodb_trx`。

---

## 10. 速记总结（30 秒）
> “InnoDB 用 **MVCC + 锁** 实现隔离。**RC**：每条语句新视图，避免脏读但有不可重复读/幻读。**RR（默认）**：事务内固定视图，配合 **Next-Key 锁** 避免锁定读下的幻读。**快照读**无行锁；**锁定读**对命中记录及间隙加锁，用于‘读后改/范围一致性’。务必使用**正确索引**，长事务要避免以免拖累 purge。”
