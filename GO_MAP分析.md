# Go map 底层数据结构详解

## 一、核心结构 overview

Go 的 map 底层由两个主要结构组成： - `hmap`：map 的顶层控制结构 -
`bmap`：bucket，用于存储 key/value

结构如下：

    hmap
     ├── buckets[]   
     └── extra overflow buckets

## 二、hmap 结构解析

定义来自 `runtime/map.go`（简化）：

``` go
type hmap struct {
    count     int
    flags     uint8
    B         uint8
    noverflow uint16
    hash0     uint32

    buckets    unsafe.Pointer
    oldbuckets unsafe.Pointer
    nevacuate  uintptr
}
```

字段说明： - `count`：当前 key 数量 - `B`：bucket 数量 = 2\^B -
`buckets`：当前使用中的 bucket 数组 - `oldbuckets`：扩容时旧桶 -
`nevacuate`：扩容迁移进度 - `hash0`：随机哈希种子

## 三、bmap（bucket）结构

每个 bucket 最多容纳 8 个 key/value。

结构：

    bucket
     ├── tophash[8]
     ├── keys[8]
     ├── values[8]
     └── overflow

tophash 是哈希高 8 bit，加速查找。

## 四、哈希分桶方式

``` text
bucketIndex = hash & ((1 << B) - 1)
```

B 控制 bucket 数量，例如 B=3 → 2\^3=8 个 bucket。

## 五、bucket 内存布局

Go 为了 cache 友好，将数据紧凑排列：

    tophash[8]
    keys...
    values...
    overflow pointer

key/value 直接 inline，不是指针，提高性能。

## 六、overflow bucket（溢出桶）

当 bucket 8 个位置占满：

    bucket0 → overflow1 → overflow2 → ...

overflow 过多会触发扩容。

## 七、扩容（rehash）机制

触发条件有两种：

### 1. 装载率太高（负载因子 \> 6.5）

    count / buckets > 6.5

→ 扩容为 2 倍。

### 2. overflow bucket 太多

数据分布不均时触发。

------------------------------------------------------------------------

## 八、渐进式扩容（incremental rehash）

扩容不会一次性完成，而是： - 每次读/写时顺便搬迁一点

机制： - `oldbuckets` 存旧桶 - `buckets` 为新桶 - `nevacuate`
指示搬迁到第几个 bucket

优点： - 不卡顿，无需 STW - 在高并发场景也更平滑

------------------------------------------------------------------------

## 九、查找 key 的过程

1.  计算 hash
2.  定位 bucket：`hash & ((1<<B)-1)`
3.  查 tophash 匹配
4.  匹配则比较 key
5.  未命中 → overflow
6.  返回 zero value + ok=false

------------------------------------------------------------------------

## 十、插入 key 的过程

1.  定位 bucket
2.  查找空位或 tombstone
3.  bucket 满 → 创建 overflow bucket
4.  插入 key/value
5.  如必要 → 扩容

------------------------------------------------------------------------

## 十一、删除 key 的过程

删除不会回收 bucket，只会留下"墓碑"（tombstone）空位。

------------------------------------------------------------------------

## 十二、map 并发写会 panic 的原因

map 写入会改变 bucket 链结构 → 并发修改容易破坏结构 → Go 直接 panic：

    fatal error: concurrent map writes

------------------------------------------------------------------------

## 十三、map 总体结构图

    hmap
    ├── count
    ├── B
    ├── buckets----+
    │              |
    │          +---v--------------------------------------+
    │          | bucket0                                   |
    │          | tophash[8]                                |
    │          | key1 key2 ... key8                        |
    │          | val1 val2 ... val8                        |
    │          | overflow -> bucket0_overflow              |
    │          +-------------------------------------------+
    │
    ├── oldbuckets (扩容时)
    └── nevacuate

------------------------------------------------------------------------

## 十四、总结（核心要点）

-   map = 哈希表 + bucket + overflow
-   bucket 每个能放 8 个 key/value
-   key/value inline 存储，提高性能
-   tophash 加速匹配
-   扩容是渐进式 rehash
-   两种扩容触发方式：load factor、overflow 多
-   map 是引用类型、非并发安全
-   删除 key 是 tombstone，不释放 bucket
-   遍历顺序故意随机化
