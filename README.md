# BEpusdt 本地补丁跟随上游

这个仓库保存本机对 BEpusdt 的改动。每天北京时间 04:00，GitHub Actions 拉取
`v03413/bepusdt` 的最新正式版，把补丁套到临时副本上并运行测试。

它不会自动替换正在运行的容器。补丁冲突或测试失败时只发通知，生产保持原样。

## 本地改动

- EasyPay 多币种收银台：`type=crypto` 创建待选币种订单。
- EasyPay 查单接口：`/api.php?act=order`，供 Sub2API 回调确认。
- Nginx TLS 终止后的收银台地址使用 `X-Forwarded-Proto`。
- TRON 节点写成 `http://` 或 `https://` 时走 TronGrid HTTP 钱包接口；`host:port` 仍走 gRPC。

## 只验证，不部署

```bash
./scripts/prepare-source.sh main /tmp/bepusdt-patch-check
cd /tmp/bepusdt-patch-check
go test ./app/handler/epay ./app/model ./app/task
```

最新正式版也可以这样检查：

```bash
release=$(./scripts/latest-release.sh)
./scripts/prepare-source.sh "$release" /tmp/bepusdt-release-check
```

## 生产注意

当前生产容器里的 HTTP 扫描程序是热替换进去的，镜像
`dmechai/bepusdt-multi-currency:local` 本身还是旧程序。不要直接重建或重启该容器，
否则会退回只支持 gRPC 的版本。磁盘空间恢复后，再用本仓库生成的源码构建新镜像。
