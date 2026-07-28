# 棋遇

一个移动端优先、可通过链接邀请好友实时对战的五子棋项目。

## 环境要求

- Go 1.23 或更高版本
- 支持 WebSocket 的现代浏览器

前端不需要安装 Node.js 或执行打包命令，Go 服务会直接提供 HTML、CSS 和 JavaScript 文件。

## 下载项目

```bash
git clone git@github.com:JasonBike/cute-gomoku.git
cd cute-gomoku
go mod download
```

## 启动项目

在项目根目录执行：

```bash
go run ./cmd/server
```

终端出现下面的提示即表示启动成功：

```text
棋遇服务已启动：http://localhost:8090
```

使用浏览器打开：

```text
http://localhost:8090
```

不要直接双击 `index.html` 进行联机对战。HTML、JavaScript 和 Go 服务必须来自同一个地址，浏览器才能连接 `/api/rooms` 和 `/ws`。

如需更换端口：

```bash
go run ./cmd/server -addr :9000
```

停止服务时，在运行服务的终端按 `Control + C`。

## 编译后运行

```bash
go build -o cute-gomoku ./cmd/server
./cute-gomoku
```

生成的 `cute-gomoku` 是当前操作系统对应的可执行文件，默认仍然监听 `8090` 端口。

## 两人联调

1. 玩家 A 点击“创建房间”。
2. 复制邀请链接。
3. 玩家 B 用另一个浏览器、无痕窗口或另一台设备打开链接。
4. 页面根据链接中的房间号自动加入，无需再次输入。
5. 双方 WebSocket 连接成功后自动进入棋局。

同一局域网内测试时，把邀请链接中的 `localhost` 换成运行服务电脑的局域网 IP，例如：

```text
http://192.168.1.20:8090/?room=7K2M8P
```

跨网络分享需要把 Go 服务部署到一台具有 HTTPS 域名的服务器。

## 运行测试

```bash
go test ./...
```

检查 WebSocket 并发读写问题：

```bash
go test -race ./...
```

## 当前联机能力

- 服务端安全随机生成六位房间号
- 一次性玩家 Token
- WebSocket 双向实时同步
- Go 服务端校验身份、轮次和落子位置
- Go 服务端判断横、竖和两条斜线五连
- 断线自动重连
- 认输
- 双方确认再来一局
- 等待房间和结束房间自动过期

房间目前保存在单个 Go 进程的内存中，服务重启后会清空。排行榜仍是前端展示数据，接入账号和数据库后再用于正式排位。
