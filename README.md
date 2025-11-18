# VideoShare - 轻量级视频分享站

一个使用 Go 语言开发的自建轻量级视频分享系统，支持 HLS 视频播放、访问控制和后台管理。

## 📋 功能特性

### 核心功能
- ✅ **视频上传** - 支持 Web 上传和服务器手工上传
- ✅ **HLS 播放** - 所有视频自动转换为 HLS 格式流畅播放
- ✅ **访问控制** - 支持公开视频和密码保护视频
- ✅ **后台管理** - 完整的视频管理、编辑、删除功能
- ✅ **权限管理** - 可设置上传权限（公开/仅管理员）
- ✅ **批量导入** - 扫描服务器目录批量导入视频

### 技术特性
- 🚀 使用 Go 1.21+ 开发，高性能低资源占用
- 💾 SQLite 数据库，零配置开箱即用
- 🎬 自动 HLS 转码，支持自适应码率
- 🔐 bcrypt 密码加密，Session 会话管理
- 📱 响应式设计，支持移动端访问

## 🛠️ 技术栈

- **后端**: Go + Gin Framework
- **数据库**: SQLite + GORM
- **视频处理**: FFmpeg
- **前端**: HTML5 + CSS3 + JavaScript + hls.js
- **认证**: gorilla/sessions + bcrypt

## 📦 安装部署

### 前置要求

1. **Go 1.21 或更高版本**
   ```bash
   go version
   ```

2. **FFmpeg 和 FFprobe**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install ffmpeg

   # macOS
   brew install ffmpeg

   # 验证安装
   ffmpeg -version
   ffprobe -version
   ```

### 快速开始

1. **克隆项目**
   ```bash
   git clone <repository-url>
   cd videoshare
   ```

2. **安装依赖**
   ```bash
   go mod tidy
   ```

3. **创建必要的目录**
   ```bash
   mkdir -p data/videos/{originals,hls} data/import
   ```

4. **初始化管理员账号**
   ```bash
   go run cmd/server/main.go -init-admin
   # 输入管理员用户名和密码
   ```

5. **启动服务**
   ```bash
   go run cmd/server/main.go
   ```

6. **访问系统**
   - 上传页面: http://localhost:8080/upload
   - 管理后台: http://localhost:8080/admin/login

### 生产环境部署

1. **编译二进制文件**
   ```bash
   go build -o videoshare cmd/server/main.go
   ```

2. **配置 Systemd 服务**

   创建 `/etc/systemd/system/videoshare.service`：
   ```ini
   [Unit]
   Description=VideoShare Service
   After=network.target

   [Service]
   Type=simple
   User=www-data
   WorkingDirectory=/opt/videoshare
   ExecStart=/opt/videoshare/videoshare
   Restart=on-failure

   [Install]
   WantedBy=multi-user.target
   ```

3. **启动服务**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable videoshare
   sudo systemctl start videoshare
   ```

4. **配置 Nginx 反向代理**

   创建 `/etc/nginx/sites-available/videoshare`：
   ```nginx
   server {
       listen 80;
       server_name video.example.com;

       # 大文件上传支持
       client_max_body_size 5G;

       # HLS 静态资源
       location /hls/ {
           alias /opt/videoshare/data/videos/hls/;
           add_header Cache-Control "public, max-age=3600";
       }

       # 代理到 Go 服务
       location / {
           proxy_pass http://127.0.0.1:8080;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
       }
   }
   ```

   启用站点：
   ```bash
   sudo ln -s /etc/nginx/sites-available/videoshare /etc/nginx/sites-enabled/
   sudo nginx -t
   sudo systemctl reload nginx
   ```

5. **配置 HTTPS（可选但推荐）**
   ```bash
   sudo apt install certbot python3-certbot-nginx
   sudo certbot --nginx -d video.example.com
   ```

## ⚙️ 配置说明

配置文件 `config.yaml`：

```yaml
server:
  port: 8080              # 服务端口
  host: "0.0.0.0"         # 监听地址

database:
  path: "./data/videoshare.db"  # 数据库文件路径

storage:
  videos_dir: "./data/videos"           # 视频存储目录
  originals_dir: "./data/videos/originals"  # 原始视频
  hls_dir: "./data/videos/hls"          # HLS 转码后文件
  import_dir: "./data/import"           # 手工上传目录

upload:
  max_size: 5368709120    # 最大上传大小 (5GB)
  allowed_types:          # 允许的视频类型
    - video/mp4
    - video/x-matroska
    - video/avi
    - video/quicktime

ffmpeg:
  path: "ffmpeg"          # FFmpeg 路径
  ffprobe_path: "ffprobe" # FFprobe 路径
  hls_time: 10            # HLS 分片时长（秒）
  hls_segment_filename: "segment_%03d.ts"

session:
  secret: "change-this-secret-key-in-production"  # Session 密钥（生产环境务必修改！）
  max_age: 86400          # Session 过期时间（秒）
```

**⚠️ 重要**: 生产环境部署前请务必修改 `session.secret` 为随机字符串！

## 📖 使用指南

### 管理员操作

1. **登录后台**
   - 访问 `/admin/login`
   - 使用初始化时创建的管理员账号登录

2. **管理视频**
   - 查看所有已上传视频
   - 编辑视频信息（标题、描述、密码保护）
   - 删除不需要的视频

3. **设置上传权限**
   - 开启：允许所有访客上传视频
   - 关闭：仅管理员可上传

4. **导入服务器视频**
   - 将视频文件放到 `data/import/` 目录
   - 在后台点击"扫描导入服务器视频"
   - 系统自动转码并入库

### 访客操作

1. **上传视频** (需开启访客上传权限)
   - 访问 `/upload`
   - 选择视频文件，填写标题描述
   - 上传完成后获得播放链接

2. **观看视频**
   - 公开视频：直接观看
   - 受保护视频：输入密码后观看

## 🗂️ 项目结构

```
videoshare/
├── cmd/server/          # 程序入口
├── internal/
│   ├── config/          # 配置管理
│   ├── models/          # 数据模型
│   ├── database/        # 数据库连接
│   ├── handler/         # HTTP 处理器
│   ├── service/         # 业务逻辑
│   ├── middleware/      # 中间件
│   └── utils/           # 工具函数
├── web/
│   ├── templates/       # HTML 模板
│   └── static/          # 静态资源
├── data/                # 数据目录
│   ├── videos/          # 视频文件
│   ├── import/          # 导入目录
│   └── videoshare.db    # 数据库
├── config.yaml          # 配置文件
├── DESIGN.md            # 设计文档
└── README.md            # 本文件
```

## 🔒 安全建议

1. **修改 Session 密钥**
   - 生产环境务必修改 `config.yaml` 中的 `session.secret`

2. **使用 HTTPS**
   - 生产环境强烈建议配置 SSL 证书

3. **限制文件大小**
   - 根据服务器性能调整 `upload.max_size`

4. **防火墙配置**
   - 只开放 80/443 端口，不要直接暴露 8080

5. **定期备份**
   - 备份 `data/` 目录和 `config.yaml`

## 🐛 故障排查

### 上传失败

1. 检查 FFmpeg 是否正确安装
   ```bash
   ffmpeg -version
   ```

2. 检查目录权限
   ```bash
   ls -la data/videos/
   ```

3. 检查磁盘空间
   ```bash
   df -h
   ```

### 播放失败

1. 检查浏览器控制台错误
2. 确认 HLS 文件是否生成
   ```bash
   ls -la data/videos/hls/
   ```

3. 检查 Nginx 配置（如使用）

### 无法登录后台

1. 确认管理员账号已创建
2. 检查 Session 配置
3. 清除浏览器 Cookie 重试

## 📝 API 文档

详细的 API 接口说明请参考 [DESIGN.md](DESIGN.md) 文档。

## 🚀 后续扩展

- [ ] 视频缩略图自动生成
- [ ] 视频下载功能
- [ ] 播放统计分析
- [ ] 视频分类和标签
- [ ] 全文搜索功能
- [ ] 批量操作优化
- [ ] Docker 容器化部署
- [ ] 多管理员支持

## 📄 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 💬 支持

如有问题，请提交 Issue 或联系项目维护者。

---

**享受你的视频分享站！** 🎉
