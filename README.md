# VideoShare - 轻量级视频分享站

一个使用 Go 语言开发的自建轻量级视频分享系统，支持 HLS 视频播放、访问控制和后台管理。

## 📋 功能特性

### 核心功能
- ✅ **视频上传** - 支持 Web 上传和服务器批量导入，上传后立即返回播放地址
- ✅ **异步转码** - 后台转码不阻塞用户，转码进度实时显示
- ✅ **智能编码** - 自动检测视频格式，兼容视频直接使用，不兼容格式才转码
- ✅ **HLS 播放** - 转换为 HLS 格式，确保主流手机和浏览器兼容
- ✅ **AI 字幕** - Whisper 自动生成字幕，支持中英双语显示
- ✅ **访问控制** - 支持公开视频和密码保护视频
- ✅ **后台管理** - 完整的视频管理、编辑、删除、导入进度显示
- ✅ **权限管理** - 可设置上传权限（公开/仅管理员）

### 技术特性
- 🚀 使用 Go 1.21+ 开发，高性能低资源占用
- 💾 SQLite 数据库，零配置开箱即用
- 🎬 智能视频转码：
  - H.264 8-bit + AAC：直接使用（快速）
  - High 10 / 10-bit / HDR / >60fps：转码为兼容格式
  - 异步处理，用户立即获得播放地址
- 🤖 AI 字幕生成：
  - Whisper API 精准识别语音
  - 自动检测语言，非中文自动翻译
  - 时间轴准确对齐，支持大文件切分
  - 保留原始 JSON 便于调试
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
   mkdir -p data/videos/{originals,hls,failed} data/import data/subtitles
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
  subtitles_dir: "./data/subtitles"     # 字幕文件目录

upload:
  max_size: 5368709120    # 最大上传大小 (5GB)
  allowed_types:          # 允许的视频类型
    - video/mp4
    - video/x-matroska
    - video/avi
    - video/quicktime
  subtitle_types:         # 允许的字幕类型
    - .srt
    - .vtt

ffmpeg:
  path: "ffmpeg"          # FFmpeg 路径
  ffprobe_path: "ffprobe" # FFprobe 路径
  hls_time: 10            # HLS 分片时长（秒）
  hls_segment_filename: "segment_%03d.ts"

session:
  secret: "change-this-secret-key-in-production"  # Session 密钥（生产环境务必修改！）
  max_age: 86400          # Session 过期时间（秒）

openai:
  api_key: "sk-..."       # OpenAI API Key（用于字幕生成）
  api_base: "https://api.openai.com/v1"  # API 地址
  whisper_model: "whisper-1"  # Whisper 模型
  translation_model: "gpt-4o-mini"  # 翻译模型

subtitle:
  chinese_color: "#FFD700"    # 中文字幕颜色
  chinese_font: "Arial"       # 中文字幕字体
  original_color: "#FFFFFF"   # 原文字幕颜色
  original_font: "Arial"      # 原文字幕字体
  font_size: "18px"           # 字幕字号
  background: "rgba(0,0,0,0.7)"  # 字幕背景
  position: "bottom"          # 字幕位置
```

**⚠️ 重要**: 生产环境部署前请务必修改 `session.secret` 为随机字符串！

## 🎯 核心特性详解

### 异步转码机制

上传视频后立即返回播放地址，转码在后台异步进行：

1. **用户体验**：
   - 上传完成即可分享链接
   - 播放页面显示"转码中"状态
   - 自动轮询转码进度
   - 完成后自动刷新播放

2. **转码状态**：
   - `pending`: 等待转码
   - `processing`: 正在转码
   - `ready`: 可以播放
   - `failed`: 转码失败（显示错误信息）

### 智能编码检测

系统自动检测视频编码，只转码不兼容的格式：

**兼容格式**（不转码，直接使用）：
- H.264 Baseline/Main/High profile（8-bit）
- yuv420p 像素格式
- AAC 音频
- ≤60 fps
- 标准色域（非 HDR）

**不兼容格式**（自动转码）：
- High 10 / High 4:2:2 / High 4:4:4 profile
- 10-bit 色深（yuv420p10le）
- HDR（bt2020、HLG、PQ）
- >60 fps
- 非 AAC 音频

转码输出：`H.264 High Profile + Level 4.1 + yuv420p + AAC 128k`

### AI 字幕生成

使用 OpenAI Whisper API 自动生成高质量字幕：

**音频预处理**：
- 自动提取音频
- 转换为 16kHz 单声道 48Kbps（减小文件，降低成本）
- >20MB 自动切分处理

**识别和翻译**：
- Whisper API 返回 JSON 格式（`verbose_json`）
- 包含精确时间戳（start/end）
- 中文内容：单语字幕
- 非中文：自动翻译为中英双语
- 使用严格 JSON 格式确保 ID 对齐

**输出文件**：
- `<slug>.vtt`: 最终字幕文件
- `<slug>_whisper.json`: Whisper 原始 JSON（便于调试）

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
   - 实时显示导入进度和状态
   - 失败的文件自动移动到 `data/videos/failed/`

5. **生成字幕**（需配置 OpenAI API Key）
   - 在视频列表中点击"生成字幕"
   - 系统自动：
     - 提取音频（16kHz 单声道 48Kbps）
     - 大文件自动切分（>20MB）
     - 调用 Whisper API 识别
     - 非中文自动翻译为中英双语
     - 时间轴精准对齐
   - 原始 Whisper JSON 保存在 `data/subtitles/<slug>_whisper.json`

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
│   ├── videos/
│   │   ├── originals/   # 原始视频文件
│   │   ├── hls/         # HLS 转码文件
│   │   └── failed/      # 导入失败的文件
│   ├── import/          # 导入目录（放置待导入视频）
│   ├── subtitles/       # 字幕文件（.vtt 和 _whisper.json）
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

## 🚀 已实现功能

- ✅ 异步上传转码
- ✅ 智能格式检测和转码
- ✅ AI 字幕生成和翻译
- ✅ 批量导入进度显示
- ✅ 转码状态实时监控

## 🔮 后续扩展

- [ ] 视频缩略图自动生成
- [ ] 视频下载功能
- [ ] 播放统计分析
- [ ] 视频分类和标签
- [ ] 全文搜索功能
- [ ] Docker 容器化部署
- [ ] 多管理员支持
- [ ] 字幕编辑器
- [ ] 视频剪辑预览

## 📄 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 💬 支持

如有问题，请提交 Issue 或联系项目维护者。

---

**享受你的视频分享站！** 🎉
