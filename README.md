# 棋遇

一个移动端优先、可以通过链接邀请好友实时对战的五子棋项目。

- 前端：Vue 3、TypeScript、Vite
- 后端：Go、WebSocket
- 生产部署：Vite 产物嵌入 Go 二进制，前端、API 和 WebSocket 共用一个端口

## 项目结构

```text
cute-gomoku/
├── web/                         # Vue 前端源码
│   ├── src/
│   │   ├── components/
│   │   ├── composables/
│   │   ├── types/
│   │   └── views/
│   └── index.html
├── internal/web/dist/           # npm run build 生成的压缩产物
├── internal/web/embed.go        # 将 dist 嵌入 Go 二进制
├── internal/game/               # 房间、WebSocket 和五连判断
├── cmd/server/                  # Go 服务入口
├── package.json
├── vite.config.ts
└── go.mod
```

## 环境要求

- Go 1.23 或更高版本
- Node.js 20 或更高版本
- npm 10 或更高版本
- 支持 WebSocket 的现代浏览器

## 下载项目

```bash
git clone git@github.com:JasonBike/cute-gomoku.git
cd cute-gomoku
```

## 直接启动

仓库已经包含构建后的 `internal/web/dist`，只运行项目时不需要安装 Node.js 依赖：

```bash
go run ./cmd/server
```

启动成功后打开：

```text
http://localhost:8090
```

更换端口：

```bash
go run ./cmd/server -addr :9000
```

用户身份、buvid、Session 和战绩默认保存在：

```text
./data/state.json
```

也可以指定其他路径：

```bash
go run ./cmd/server -data /var/lib/cute-gomoku/state.json
```

服务会在启动时加载该文件，并在身份或资料发生变化时加锁、写入临时文件，再原子替换正式文件。线上需要保证数据目录可写并放在持久化磁盘中；当前 JSON 存储只支持单个 Go 服务进程写入。

不要直接双击 HTML 文件。页面、API 和 WebSocket 都由 Go 服务提供。

## 前端开发

首次安装依赖：

```bash
npm ci
```

终端一启动 Go API 和 WebSocket：

```bash
go run ./cmd/server
```

终端二启动 Vite 热更新服务：

```bash
npm run dev
```

开发时打开：

```text
http://localhost:5173
```

Vite 会把 `/api` 和 `/ws` 自动代理到 `http://127.0.0.1:8090`。

## 生产构建

修改 Vue、TypeScript 或 CSS 后，必须重新生成前端产物：

```bash
npm ci
npm run build
```

构建命令会依次执行：

1. `vue-tsc --noEmit`：TypeScript 类型检查。
2. `vite build`：压缩 JavaScript 和 CSS。
3. 将产物写入 `internal/web/dist`。

产物结构类似：

```text
internal/web/dist/
├── index.html
└── assets/
    ├── index-[hash].js
    └── index-[hash].css
```

然后编译 Go：

```bash
go build -trimpath -ldflags="-s -w" -o cute-gomoku ./cmd/server
```

生成的 `cute-gomoku` 已经包含 `index.html`、压缩后的 JS 和 CSS。线上只需要部署这个二进制，不需要另外复制 `dist`。

本机运行生产二进制：

```bash
./cute-gomoku -addr :8090
```

## 构建 Linux 部署包

在 macOS 上交叉编译 Linux AMD64：

```bash
mkdir -p release
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" \
  -o release/cute-gomoku-linux-amd64 ./cmd/server
```

Linux ARM64：

```bash
mkdir -p release
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags="-s -w" \
  -o release/cute-gomoku-linux-arm64 ./cmd/server
```

把对应二进制上传到服务器，例如：

```text
/opt/cute-gomoku/cute-gomoku
```

## systemd 服务

创建 `/etc/systemd/system/cute-gomoku.service`：

```ini
[Unit]
Description=Cute Gomoku
After=network.target

[Service]
Type=simple
User=gomoku
Group=gomoku
WorkingDirectory=/opt/cute-gomoku
ExecStart=/opt/cute-gomoku/cute-gomoku -addr 127.0.0.1:8090 -data /var/lib/cute-gomoku/state.json
Restart=always
RestartSec=3
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

加载并启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cute-gomoku
sudo systemctl status cute-gomoku
```

查看日志：

```bash
journalctl -u cute-gomoku -f
```

## Nginx 反向代理

线上必须正确转发 WebSocket。示例配置：

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name gomoku.example.com;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 75s;
    }
}
```

配置域名证书并启用 HTTPS 后，浏览器会自动使用 `wss://` 连接 WebSocket。公网环境不要使用明文 HTTP/WS。

## 部署检查

健康检查：

```bash
curl https://gomoku.example.com/api/health
```

预期结果：

```json
{"status":"ok"}
```

双人验证：

1. 玩家 A 创建房间并复制邀请链接。
2. 玩家 B 使用另一个浏览器、无痕窗口或另一台设备打开链接。
3. 页面根据 URL 房间号自动加入。
4. 双方连接后自动进入棋局。
5. 刷新页面后通过 URL 房间号和本地 Token 恢复原来的座位。

## 运行测试

```bash
go test ./...
npm run typecheck
```

检查 WebSocket 并发读写：

```bash
go test -race ./...
```

完整生产构建检查：

```bash
npm run build
go test ./...
go build ./cmd/server
```

## 当前限制

- 房间数据保存在单个 Go 进程内存中。
- 服务重启后，进行中的房间会清空。
- 当前只能部署一个 Go 实例；多实例需要使用 Redis 共享房间状态。
- 排行榜目前仍是展示数据，尚未接入账号和持久化积分。
