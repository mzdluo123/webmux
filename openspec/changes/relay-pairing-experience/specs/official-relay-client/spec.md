# Spec Delta

## REMOVED Requirements

### Requirement: 仅全新主机配对

**Reason**: 客户端规格接近治理行数上限，配对生命周期集中到同主题规格。
**Migration**: 完整要求与场景迁移至 official-relay-pairing，保留 v2 全新绑定约束。

### Requirement: 独立持久机器身份

**Reason**: 机器身份属于配对边界，与授权流程一起维护。
**Migration**: 迁移至 official-relay-pairing；旧绑定仍存在时同机请求遵循已生效的冲突语义，已授权删除后才可全新绑定。

### Requirement: 设置操作与删除绑定

**Reason**: 页内配对、启停与删除操作集中维护，避免跨规格重复。
**Migration**: 完整要求与场景迁移至 official-relay-pairing，增加自动查询与操作回执。
