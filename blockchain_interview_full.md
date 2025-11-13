# 区块链开发全套面试题与知识体系（Markdown 完整版）

## 🧩 目录
1. 区块链基础
2. 共识机制
3. Solidity 面试题
4. Web3 / Ethers.js 面试题
5. DeFi 高频面试题
6. NFT / ERC 标准面试题
7. Layer2 / Rollup 面试题
8. 区块链安全（智能合约审计）
9. 节点 / RPC / 链基础架构
10. 系统设计面试题
11. 手写代码题（Solidity）
12. Web3 高级八股文总结

---

# 1. 区块链基础

## 1.1 什么是区块链？
- 去中心化账本
- 不可篡改
- 全网共识

## 1.2 区块链分类
- 公链（Ethereum、BTC）
- 私链（企业内部）
- 联盟链（Fabric、BCOS）

## 1.3 关键属性
- 去中心化
- 可追溯
- 分布式存储
- 拜占庭容错

---

# 2. 共识机制

## 2.1 共识机制表格对比

| 共识机制 | 性能 | 能耗 | 去中心化 | 最终性 | 场景 |
|---------|------|--------|------------|--------------|--------|
| POW | 低 | 高 | 高 | 弱（可能重组） | BTC |
| POS | 中 | 低 | 高 | 强 | ETH2 |
| DPOS | 高 | 低 | 中低 | 强 | EOS、TRON |
| PBFT | 高 | 极低 | 低 | 强 | 企业链 |
| PoA | 极高 | 极低 | 低 | 强 | BSC |
| PoH | 极高 | 中 | 中 | 中 | Solana |
| ZK Rollup | 高 | 低 | 高 | 强 | zkSync |
| Optimistic | 中 | 低 | 高 | 延迟 | Arbitrum |

---

# 3. Solidity 面试题

## 3.1 memory / storage / calldata
| 类型 | 位置 | 可写？ | 用途 |
|------|-------|---------|---------|
| memory | 内存 | 可写 | 内部变量 |
| storage | 链上 | 可写 | 状态变量 |
| calldata | 只读 | 不可写 | external 参数 |

## 3.2 重入攻击防御
- checks-effects-interactions
- ReentrancyGuard
- 不使用 .call.value() 发送 ETH

## 3.3 delegatecall 风险
- 使用当前合约的存储布局
- 合约升级必须保持 storage slot 一致

## 3.4 如何升级合约？
- Transparent Proxy
- UUPS
- Beacon Proxy
- Diamond（EIP-2535）

## 3.5 require / assert / revert 区别
- require：用户输入错误
- assert：永不应失败，失败即 bug
- revert：主动回滚

---

# 4. Web3 / Ethers.js 面试题

## 4.1 监听事件
```js
contract.on("Transfer", (from, to, amount) => {})
```

## 4.2 ABI 是什么？
- 合约的接口说明（函数、事件编码规则）
- 前端通过 ABI 调用链上合约

## 4.3 EOA vs Contract Account
| 类型 | 私钥 | 是否能发交易 | 是否可执行逻辑 |
|------|-------|--------------------|-----------------------|
| EOA | 有 | 能 | 否 |
| Contract | 无 | 只能被动触发 | 有 |

## 4.4 Transaction 和 Call 区别
- call 只读，不消耗 gas
- sendTransaction 会改变状态

---

# 5. DeFi 高频面试题

## 5.1 什么是 AMM？
自动做市商  
Uniswap V2 公式：

```
x * y = k
```

## 5.2 Impermanent Loss（无常损失）
LP 在双币池中因价格偏离造成的损失。

## 5.3 Uniswap v3 集中流动性
LP 选择价格区间提供流动性 → 收益更高。

## 5.4 DeFi 常见攻击
- 闪电贷攻击
- 重入攻击
- oracle manipulation

---

# 6. NFT / ERC 标准

## 6.1 ERC-20
同质化代币接口。

## 6.2 ERC-721
独一无二 NFT。

## 6.3 ERC-1155
多资产合一标准（游戏常用）。

## 6.4 Blind Box / Reveal
- 未揭露：统一 metadata
- 开盲盒后 reveal：分配真实 metadata

---

# 7. Layer2 / Rollup

## 7.1 Optimistic Rollup
- 默认正确
- 7 天挑战窗口
- Arbitrum / Optimism

## 7.2 ZK Rollup
- zk 证明有效性
- 提交即 finality
- zkSync / StarkNet

## 7.3 State Channel
- 适用于高频游戏

---

# 8. 区块链安全（审计）

## 8.1 常见漏洞
- 重入
- delegatecall 注入
- 权限控制漏洞
- 伪随机数漏洞
- 整数溢出

## 8.2 随机数正确做法
使用 Chainlink VRF。

## 8.3 价格预言机攻击
利用操控 AMM 池价格获利。

---

# 9. 节点 / RPC / 链架构

## 9.1 全节点、轻节点、归档节点

| 类型 | 内容 | 用途 |
|-------|---------|---------|
| Full Node | 当前+历史链状态 | DApp 后端 |
| Light Node | 区块头 | 钱包、移动端 |
| Archive Node | 所有状态 | 浏览器、分析工具 |

## 9.2 什么是 Finality？
- 区块不可逆状态  
- POW 比较弱  
- POS 强一致

---

# 10. 系统设计面试题

## 10.1 如何设计 10 万 TPS 区块链？
- BFT 共识（Hotstuff / Tendermint）
- Sharding
- zkRollup
- 状态压缩
- 并行 EVM（SVM）

## 10.2 Web3 登录系统设计
- 使用 EIP-4361（Sign-In with Ethereum）
- 通过签名 + nonce 登录
- 后端不存密码

---

# 11. 手写 Solidity 面试题

## 11.1 写一个 ERC20（简化）
```solidity
contract Token {
    mapping(address => uint) public balance;

    function transfer(address to, uint amount) external {
        require(balance[msg.sender] >= amount);
        balance[msg.sender] -= amount;
        balance[to] += amount;
    }
}
```

## 11.2 写一个防重入 EtherBank
```solidity
contract Bank {
    mapping(address => uint) public bal;
    bool locked;

    modifier nonReentrant {
        require(!locked);
        locked = true; _;
        locked = false;
    }

    function withdraw() external nonReentrant {
        uint amount = bal[msg.sender];
        bal[msg.sender] = 0;
        (bool ok,) = msg.sender.call{value: amount}("");
        require(ok);
    }
}
```

---

# 12. Web3 高级八股文总结

- EVM 工作原理  
- 编译过程（Sol → Yul → Bytecode）  
- Merkle Tree / Patricia Trie  
- Zero Knowledge 简述  
- L1 / L2 数据可用性  
- MEV / Sandwich Attack  
- Gas 计算规则  
- RPC 工作机制  
- 区块结构：Header / Body / Receipt  
- Event Log 的 Bloom Filter

---

