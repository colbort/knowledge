# Redis 数据类型
## 1. 字符串（String）
字符串是 Redis 中最基本的数据类型，可以包含任何类型的数据，比如整数、浮点数、二进制数据（如图片或文件内容）。

**常用命令：**
- SET：设置一个键的值。
- GET：获取指定键的值。
- INCR：增加键的整数值。
- DECR：减少键的整数值。
- APPEND：追加字符串到指定键的值。

**示例：**
```bash
SET mykey "Hello, Redis!"
GET mykey  # 返回 "Hello, Redis!"
INCR counter  # counter 值增加 1
```

## 2. 哈希（Hash）
哈希是一个键值对集合，适用于存储对象类型的数据。每个哈希都有一个键，可以包含多个字段和值。

**常用命令：**
- HSET：设置哈希表中字段的值。
- HGET：获取哈希表中指定字段的值。
- HGETALL：获取哈希表中的所有字段和值。
- HDEL：删除哈希表中的字段。

**示例：**
```bash
HSET user:1000 name "John" age 30
HGET user:1000 name  # 返回 "John"
HGETALL user:1000  # 返回所有字段和值
```

## 3. 列表（List）
列表是一个有序的字符串集合，可以进行操作，如推送（push）或弹出（pop）元素。列表支持从两端进行操作。

**常用命令：**
- LPUSH：将一个或多个值插入列表的左边。
- RPUSH：将一个或多个值插入列表的右边。
- LPOP：从列表的左边弹出一个值。
- RPOP：从列表的右边弹出一个值。
- LRANGE：获取列表的指定范围的元素。

**示例：**
```bash
LPUSH mylist "A" "B" "C"
LRANGE mylist 0 -1  # 返回 ["C", "B", "A"]
RPOP mylist  # 返回 "A"
```

## 4. 集合（Set）
集合是一个无序的字符串集合，其中每个元素都是唯一的。适合用于存储不重复的数据，支持集合运算如交集、并集和差集。

**常用命令：**
- SADD：向集合添加一个或多个成员。
- SMEMBERS：获取集合中的所有成员。
- SISMEMBER：检查某个成员是否在集合中。
- SPOP：从集合中移除并返回一个随机成员。

**示例：**
```bash
SADD myset "apple" "banana" "orange"
SMEMBERS myset  # 返回 ["apple", "banana", "orange"]
SISMEMBER myset "apple"  # 返回 1 (true)
SPOP myset  # 随机移除一个元素
```

## 5. 有序集合（Sorted Set）
有序集合类似于集合，但每个元素都会关联一个分数（score）。Redis 会根据分数对元素进行排序。适合用于排名系统、权重系统等。

**常用命令：**
- ZADD：向有序集合添加一个或多个成员，或者更新已存在成员的分数。
- ZRANGE：获取有序集合指定范围的成员。
- ZREM：从有序集合中删除一个或多个成员。
- ZINCRBY：增加有序集合中成员的分数。

**示例：**
```bash
ZADD leaderboard 100 "Alice" 200 "Bob" 150 "Charlie"
ZRANGE leaderboard 0 -1  # 返回 ["Alice", "Charlie", "Bob"]
ZINCRBY leaderboard 50 "Alice"  # Alice 的分数增加 50
```

## 6. 位图（Bitmap）
位图是 Redis 中的一种非常高效的存储方式，用于处理大量的二进制数据，尤其适用于做大量的 true/false 类型的记录。Redis 位图实际上是通过对字符串进行位操作来实现的。

**常用命令：**
- SETBIT：设置指定位置的位（0 或 1）。
- GETBIT：获取指定位置的位。
- BITCOUNT：计算位图中值为1的位的数量。

**示例：**
```bash
SETBIT mybitmap 7 1  # 将第7位设置为1
GETBIT mybitmap 7  # 返回 1
BITCOUNT mybitmap  # 返回当前位图中1的数量
```

## 7. HyperLogLog
HyperLogLog 是一种基于概率的数据结构，用于做基数统计。它的特点是能够用非常少的内存来估算大型数据集的不同元素的数量。

**常用命令：**
- PFADD：将指定的元素添加到 HyperLogLog 中。
- PFCOUNT：返回 HyperLogLog 中不同元素的估算数量。

**示例：**
```bash
PFADD myhll "apple" "banana" "cherry"
PFCOUNT myhll  # 返回估算的不同元素数量
```

## 8. 地理空间（Geo）
Redis 提供了对地理空间数据的支持，可以存储地理位置并进行查询。

**常用命令：**
- GEOADD：将一个或多个地理位置添加到 Redis 中。
- GEODIST：计算两个地理位置之间的距离。
- GEORADIUS：查询某个范围内的地理位置。

**示例：**
```bash
GEOADD cities 13.361389 38.115556 "Palermo" 15.087269 37.502669 "Catania"
GEODIST cities "Palermo" "Catania"  # 计算两个位置的距离
```

## 总结
Redis 提供了多种灵活的数据结构来满足不同的业务需求。通过使用不同的 Redis 数据类型，开发者可以在内存数据库中进行高效的数据存储、处理和查询。不同的数据类型适用于不同的场景，例如：

- 字符串适合存储简单的键值对数据；
- 哈希适合存储对象数据；
- 列表和集合适合存储有序和无序的集合数据；
- 有序集合适合处理带有排序的数据；
- 位图适合进行高效的位级操作；
- HyperLogLog 适合进行基数统计；
- Geo 适合进行地理位置查询。



# Redis 持久化

Redis 持久化是 Redis 提供的一项功能，用于将数据从内存持久化到磁盘，确保即使在 Redis 重启时，数据也不会丢失。Redis 提供了两种主要的持久化方式：RDB（快照持久化）和AOF（追加文件持久化），你可以根据业务需求选择适合的持久化方式。

## 1. RDB（Redis 数据库快照）持久化
RDB 是 Redis 的一种持久化方式，它通过在指定的时间间隔内生成数据的快照来保存数据。

### 工作原理：
- Redis 会在内存中创建一个时间点的快照，将所有数据保存到磁盘上的 RDB 文件中。
- 当 Redis 进程退出时，RDB 会保留最后一个时间点的快照，重启时加载该文件来恢复数据。

### 配置：
在 redis.conf 配置文件中，你可以通过设置 save 指令来配置 RDB 的生成条件。每当满足某个条件时，Redis 会创建 RDB 快照。例如：

```bash
save 900 1   # 900秒（15分钟）内，如果有至少 1 个键被修改，则生成快照
save 300 10  # 300秒（5分钟）内，如果有至少 10 个键被修改，则生成快照
save 60 10000 # 60秒内，如果有至少 10000 个键被修改，则生成快照
```
### 优缺点：
**优点：**
- 生成 RDB 文件后不会影响 Redis 的性能。
- 可以进行完整备份，适合做数据备份。
- 恢复数据速度较快。
**缺点：**
— 数据不完全实时，快照之间的数据会丢失，可能会导致数据丢失。

## 2. AOF（Append-Only File）追加文件持久化
AOF 是另一种持久化方式，它通过记录所有写命令到日志文件中来保存数据。每次执行修改操作时，Redis 会将相应的写命令（如 SET, INCR 等）追加到 AOF 文件末尾。

### 工作原理：
- 每当 Redis 执行写操作时，它会将相应的命令记录到 AOF 文件中。
- 在 Redis 重启时，Redis 会读取 AOF 文件并按照文件中的命令重新执行，从而恢复数据。
### 配置：
在 redis.conf 配置文件中，你可以配置 AOF 相关的参数：
```bash
appendonly yes         # 启用 AOF 持久化
appendfsync everysec   # 每秒同步一次 AOF 文件
```
### AOF 还支持几种同步方式，可以通过 appendfsync 参数配置：
- always：每次写操作后都会同步 AOF 文件（性能较差）。
- everysec：每秒同步一次（常用设置，性能较好）。
- no：不主动同步（AOF 写入速度较快，但可能丢失数据）。
### 优缺点：
**优点：**
- 相比 RDB，AOF 适合实时性要求较高的应用，可以最大限度地减少数据丢失。
- 即使 Redis 崩溃，丢失的数据也较少（除非 AOF 文件没有同步）。
**缺点：**
- AOF 文件较大，恢复时需要执行更多的写命令，恢复速度比 RDB 慢。
- AOF 文件随着时间推移可能变得非常大，需要进行重写（即 BGREWRITEAOF）来优化 AOF 文件。

## 3. 混合持久化（RDB + AOF）
Redis 还支持同时启用 RDB 和 AOF 持久化方式，以充分发挥两者的优势。
- 在这种模式下，Redis 会定期保存 RDB 快照，同时也会记录 AOF 日志。
- 启用混合持久化时，可以将数据的恢复速度和实时性结合起来，确保数据的持久化和恢复效率。

### 配置：
在 redis.conf 中启用 AOF 持久化，同时启用 RDB 快照：
```bash
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfsync everysec
```

## 4. 持久化选择和恢复策略
RDB 适合：定期备份数据，数据丢失容忍度较高。
- AOF 适合：实时性要求高的应用，需要最小化数据丢失。
- 混合持久化适合：既需要数据恢复速度又要求较低的实时性，结合 RDB 和 AOF 的优点。

## 5. 持久化的后台操作
Redis 还提供了一些后台命令用于处理 RDB 和 AOF 文件，比如：
- BGSAVE：在后台生成 RDB 快照。
- BGREWRITEAOF：在后台重写 AOF 文件以减小其大小。

**总结**
- RDB 和 AOF 各有优缺点，适用于不同的场景。
- 如果希望高性能的同时容忍一定的数据丢失，使用 RDB。
- 如果实时性要求较高，需要最大程度减少数据丢失，则使用 AOF。
- 可以结合使用 RDB 和 AOF，结合两者的优点来提高持久化的可靠性和性能。
