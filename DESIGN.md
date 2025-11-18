# VideoShare 系统设计文档

## 1. 系统架构设计

### 1.1 整体架构

```
┌─────────────┐
│   Nginx     │ (反向代理 + 静态资源)
└──────┬──────┘
       │
┌──────▼──────┐
│  Go 后端服务 │
├─────────────┤
│ - HTTP 路由 │
│ - 业务逻辑  │
│ - 访问控制  │
│ - HLS 转换  │
└──────┬──────┘
       │
   ┌───┴───┬──────────┬─────────┐
   │       │          │         │
┌──▼──┐ ┌─▼──┐  ┌────▼────┐ ┌──▼────┐
│SQLite│ │文件系统│ │HLS 处理│ │Session│
└─────┘ └────┘  └─────────┘ └───────┘
```

### 1.2 技术栈

- **后端语言**: Go 1.21+
- **Web 框架**: Gin (轻量级、高性能)
- **数据库**: SQLite (轻量级、免配置)
- **ORM**: GORM
- **Session**: gorilla/sessions
- **密码加密**: bcrypt
- **视频处理**: FFmpeg (通过 exec 调用)
- **前端**: 原生 HTML/CSS/JavaScript + hls.js

### 1.3 目录结构

```
videoshare/
├── cmd/
│   └── server/
│       └── main.go              # 程序入口
├── internal/
│   ├── config/
│   │   └── config.go            # 配置管理
│   ├── models/
│   │   ├── video.go             # 视频模型
│   │   ├── admin.go             # 管理员模型
│   │   └── setting.go           # 系统设置模型
│   ├── database/
│   │   └── db.go                # 数据库连接
│   ├── handler/
│   │   ├── video.go             # 视频相关路由处理
│   │   ├── admin.go             # 管理员路由处理
│   │   ├── upload.go            # 上传路由处理
│   │   └── auth.go              # 认证相关处理
│   ├── service/
│   │   ├── video_service.go     # 视频业务逻辑
│   │   ├── hls_service.go       # HLS 转换服务
│   │   ├── import_service.go    # 视频导入服务
│   │   └── auth_service.go      # 认证服务
│   ├── middleware/
│   │   ├── auth.go              # 认证中间件
│   │   └── session.go           # Session 中间件
│   └── utils/
│       ├── hash.go              # 密码哈希工具
│       └── slug.go              # Slug 生成工具
├── web/
│   ├── templates/
│   │   ├── upload.html          # 上传页面
│   │   ├── watch.html           # 播放页面
│   │   ├── admin/
│   │   │   ├── login.html       # 管理员登录页
│   │   │   ├── dashboard.html   # 管理主页
│   │   │   └── video_edit.html  # 视频编辑页
│   │   └── password.html        # 密码输入页
│   └── static/
│       ├── css/
│       ├── js/
│       │   └── hls.min.js       # hls.js 库
│       └── images/
├── data/
│   ├── videos/                  # 视频存储目录
│   │   ├── originals/           # 原始上传视频
│   │   └── hls/                 # HLS 转换后的文件
│   ├── import/                  # 手工上传目录
│   └── videoshare.db            # SQLite 数据库文件
├── go.mod
├── go.sum
├── README.md
├── DESIGN.md                    # 本文档
└── requirement.md               # 需求文档
```

---

## 2. 数据库设计

### 2.1 视频表 (videos)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | INTEGER PRIMARY KEY | 自增主键 |
| slug | VARCHAR(20) UNIQUE | 播放链接标识，唯一 |
| title | VARCHAR(255) | 视频标题 |
| description | TEXT | 视频描述 |
| original_filename | VARCHAR(255) | 原始文件名 |
| original_path | VARCHAR(500) | 原始视频路径 |
| hls_path | VARCHAR(500) | HLS m3u8 文件路径 |
| duration | INTEGER | 视频时长（秒） |
| file_size | BIGINT | 文件大小（字节） |
| is_password_protected | BOOLEAN | 是否需要密码 |
| password_hash | VARCHAR(255) | 密码哈希（可为空） |
| upload_type | VARCHAR(20) | 上传类型：web/import/admin |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### 2.2 管理员表 (admins)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | INTEGER PRIMARY KEY | 自增主键 |
| username | VARCHAR(50) UNIQUE | 用户名 |
| password_hash | VARCHAR(255) | 密码哈希 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### 2.3 系统设置表 (settings)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | INTEGER PRIMARY KEY | 自增主键 |
| key | VARCHAR(50) UNIQUE | 配置键 |
| value | TEXT | 配置值 |
| updated_at | DATETIME | 更新时间 |

**默认配置项：**
- `upload_permission`: "public" / "admin" (默认 "public")

---

## 3. API 设计

### 3.1 公开接口

#### 上传页面
- `GET /upload` - 显示上传页面（根据权限设置判断）

#### 视频上传
- `POST /api/upload` - 上传视频
  - Form Data: `video` (文件), `title` (可选), `description` (可选)
  - Response: `{"slug": "abc123", "url": "/v/abc123"}`

#### 视频播放
- `GET /v/:slug` - 视频播放页面
- `GET /api/video/:slug/info` - 获取视频信息（需验证密码）
- `POST /api/video/:slug/verify` - 验证视频密码
  - Body: `{"password": "xxx"}`
  - Response: `{"success": true}`

#### HLS 资源
- `GET /hls/:slug/playlist.m3u8` - 获取 m3u8 文件（需验证）
- `GET /hls/:slug/:segment` - 获取 ts 分片（需验证）

### 3.2 管理员接口

#### 认证
- `GET /admin/login` - 登录页面
- `POST /api/admin/login` - 登录接口
  - Body: `{"username": "admin", "password": "xxx"}`
- `POST /api/admin/logout` - 退出登录

#### 视频管理
- `GET /admin/dashboard` - 管理主页（视频列表）
- `GET /api/admin/videos` - 获取视频列表（JSON）
- `GET /api/admin/videos/:id` - 获取单个视频详情
- `PUT /api/admin/videos/:id` - 更新视频信息
  - Body: `{"title": "", "description": "", "is_password_protected": true, "password": ""}`
- `DELETE /api/admin/videos/:id` - 删除视频

#### 导入管理
- `POST /api/admin/import` - 触发扫描导入
  - Response: `{"imported": 3, "skipped": 1, "failed": 0, "messages": []}`

#### 系统设置
- `GET /api/admin/settings` - 获取系统设置
- `PUT /api/admin/settings` - 更新系统设置
  - Body: `{"upload_permission": "public"}`

---

## 4. 核心模块设计

### 4.1 HLS 转换服务

**功能职责：**
- 将上传的视频转换为 HLS 格式
- 生成 m3u8 播放列表和 ts 分片文件
- 提取视频元信息（时长、分辨率等）

**实现方案：**
```go
type HLSService struct {
    ffmpegPath string
    outputDir  string
}

// ConvertToHLS 转换视频为 HLS 格式
func (s *HLSService) ConvertToHLS(inputPath, outputDir string) (*HLSInfo, error) {
    // 1. 创建输出目录
    // 2. 调用 FFmpeg 转换
    //    ffmpeg -i input.mp4 -c:v libx264 -c:a aac -hls_time 10 -hls_list_size 0 -f hls output.m3u8
    // 3. 解析输出结果，获取元信息
    // 4. 返回 HLS 信息
}

// GetVideoInfo 获取视频元信息
func (s *HLSService) GetVideoInfo(inputPath string) (*VideoInfo, error) {
    // 使用 ffprobe 获取视频信息
    // ffprobe -v error -show_entries format=duration,size -of json input.mp4
}
```

### 4.2 视频导入服务

**功能职责：**
- 扫描指定目录的视频文件
- 检查文件是否已导入（去重）
- 批量导入视频到系统

**实现方案：**
```go
type ImportService struct {
    importDir string
    videoService *VideoService
    hlsService   *HLSService
}

// ScanAndImport 扫描并导入视频
func (s *ImportService) ScanAndImport() (*ImportResult, error) {
    // 1. 扫描 importDir 目录
    // 2. 过滤视频文件（mp4, avi, mkv, mov 等）
    // 3. 对每个文件：
    //    - 检查是否已导入（通过文件名或哈希）
    //    - 移动/复制到视频存储目录
    //    - 调用 HLS 转换
    //    - 创建数据库记录
    // 4. 返回导入结果统计
}
```

### 4.3 访问控制服务

**功能职责：**
- 验证视频访问权限
- 管理视频密码验证 session
- 控制 HLS 资源访问

**实现方案：**
```go
type AccessControl struct {
    sessionStore sessions.Store
}

// VerifyVideoAccess 验证视频访问权限
func (a *AccessControl) VerifyVideoAccess(c *gin.Context, video *models.Video) error {
    // 1. 如果视频不需要密码，直接通过
    // 2. 如果需要密码，检查 session 中是否已验证
    // 3. 如果未验证，返回需要密码错误
}

// VerifyPassword 验证视频密码
func (a *AccessControl) VerifyPassword(c *gin.Context, video *models.Video, password string) error {
    // 1. 验证密码是否正确
    // 2. 如果正确，在 session 中记录已验证
    // 3. 返回验证结果
}
```

### 4.4 认证中间件

**功能职责：**
- 保护管理员路由
- 验证管理员登录状态

**实现方案：**
```go
// AuthRequired 管理员认证中间件
func AuthRequired() gin.HandlerFunc {
    return func(c *gin.Context) {
        session, _ := sessionStore.Get(c.Request, "admin-session")
        adminID, ok := session.Values["admin_id"]
        if !ok || adminID == nil {
            c.Redirect(302, "/admin/login")
            c.Abort()
            return
        }
        c.Set("admin_id", adminID)
        c.Next()
    }
}
```

---

## 5. 安全设计

### 5.1 密码安全

- 使用 bcrypt 进行密码哈希存储
- 盐值由 bcrypt 自动生成
- 成本因子设置为 10（默认）

```go
// HashPassword 生成密码哈希
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

### 5.2 Session 管理

- 使用 gorilla/sessions
- Session 存储方式：Cookie（加密）
- Session 过期时间：24小时
- 管理员 session 和视频访问 session 分开管理

### 5.3 访问控制

- 所有管理员接口需要认证
- HLS 资源访问需要验证视频权限
- 上传接口根据系统设置控制访问

### 5.4 文件上传安全

- 限制上传文件大小（如 5GB）
- 验证文件类型（MIME type）
- 使用随机文件名存储，避免路径遍历
- 上传目录与程序目录分离

---

## 6. 前端设计

### 6.1 上传页面 (upload.html)

**功能：**
- 文件选择器（支持拖拽）
- 标题、描述输入框
- 上传进度条
- 上传完成后显示播放链接

**技术：**
- 使用 FormData 上传文件
- XMLHttpRequest 监听上传进度

### 6.2 播放页面 (watch.html)

**功能：**
- 视频标题、描述展示
- HLS 视频播放器
- 密码输入表单（需要时显示）

**技术：**
- 使用 hls.js 播放 HLS 视频
- Video.js 或原生 `<video>` 标签

### 6.3 管理后台 (admin/dashboard.html)

**功能：**
- 视频列表展示（表格）
- 编辑、删除操作按钮
- 导入扫描按钮
- 上传权限设置开关
- 分页功能

**技术：**
- 使用 AJAX 调用 API
- 简单的 CSS 框架（如 Bootstrap 或自定义样式）

---

## 7. 部署方案

### 7.1 生产环境部署

**Nginx 配置示例：**
```nginx
server {
    listen 80;
    server_name video.example.com;

    # 静态资源直接提供
    location /hls/ {
        alias /var/www/videoshare/data/videos/hls/;
        add_header Cache-Control "public, max-age=3600";
    }

    # 其他请求转发到 Go 服务
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # 支持大文件上传
        client_max_body_size 5G;
    }
}
```

### 7.2 系统服务配置

**Systemd 服务文件：** `/etc/systemd/system/videoshare.service`
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

### 7.3 初始化脚本

首次运行时需要：
1. 创建数据目录
2. 初始化数据库
3. 创建默认管理员账号

```bash
./videoshare --init-admin
# 输入管理员用户名和密码
```

---

## 8. 配置管理

### 8.1 配置文件 (config.yaml)

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  path: "./data/videoshare.db"

storage:
  videos_dir: "./data/videos"
  originals_dir: "./data/videos/originals"
  hls_dir: "./data/videos/hls"
  import_dir: "./data/import"

upload:
  max_size: 5368709120  # 5GB
  allowed_types:
    - video/mp4
    - video/x-matroska
    - video/avi
    - video/quicktime

ffmpeg:
  path: "ffmpeg"
  ffprobe_path: "ffprobe"
  hls_time: 10  # 每个分片时长（秒）
  hls_segment_filename: "segment_%03d.ts"

session:
  secret: "your-secret-key-change-this"
  max_age: 86400  # 24小时
```

---

## 9. 开发计划

### 9.1 第一阶段：基础框架

- [ ] 项目初始化，目录结构搭建
- [ ] 数据库模型和连接
- [ ] 基本路由和中间件
- [ ] 配置管理

### 9.2 第二阶段：核心功能

- [ ] HLS 转换服务实现
- [ ] 视频上传功能
- [ ] 视频播放功能
- [ ] 访问控制实现

### 9.3 第三阶段：管理功能

- [ ] 管理员认证
- [ ] 视频管理 CRUD
- [ ] 视频导入功能
- [ ] 系统设置管理

### 9.4 第四阶段：前端界面

- [ ] 上传页面
- [ ] 播放页面
- [ ] 管理后台界面

### 9.5 第五阶段：测试和优化

- [ ] 功能测试
- [ ] 安全测试
- [ ] 性能优化
- [ ] 部署文档

---

## 10. 潜在扩展功能（后续版本）

- 视频缩略图生成
- 视频下载功能
- 播放统计
- 视频分类/标签
- 搜索功能
- 批量导入优化（后台任务队列）
- API 密钥访问
- 多管理员支持
- 访问日志
