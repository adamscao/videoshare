# VideoShare Implementation Guide

## 已完成功能 ✅

### 1. 核心配置和数据库
- ✅ 添加 OpenAI API 配置（Whisper + Translation）
- ✅ 添加字幕样式配置（颜色、字体等）
- ✅ 添加 GitHub URL 配置
- ✅ Video 模型添加 `subtitle_path` 字段
- ✅ 自动创建字幕目录

### 2. 字幕服务 (Whisper + Translation)
- ✅ Whisper API 集成，自动生成字幕
- ✅ OpenAI 翻译 API 集成
- ✅ 自动检测语言（中文 vs 非中文）
- ✅ 生成双语字幕（原文 + 中文）
- ✅ SRT 转 VTT 格式
- ✅ 上传字幕文件支持

### 3. CLI 管理工具 (`videocli`)
- ✅ 列出所有视频
- ✅ 设置视频密码
- ✅ 设置视频公开/私密

### 4. API 端点
- ✅ `POST /api/admin/videos/:id/generate-subtitle` - 生成字幕
- ✅ `POST /api/admin/videos/:id/upload-subtitle` - 上传字幕文件
- ✅ 后台字幕列表可点击跳转

---

## 待完成前端改进 🚧

### 1. 上传页面改进 (`web/templates/upload.html`)

需要添加的功能：

#### a) 页面头部改进
```html
<!-- 在 <body> 开始后添加 -->
<div class="header">
    <h1>视频分享平台</h1>
    <div class="header-actions">
        <a href="{{ .githubURL }}" target="_blank" class="github-link">
            <svg><!-- GitHub icon SVG --></svg>
        </a>
        <button class="lang-btn" onclick="switchLang('en')">English</button>
        <button class="lang-btn active" onclick="switchLang('zh')">中文</button>
    </div>
</div>
```

#### b) 密码保护选项
```html
<!-- 在description输入框后添加 -->
<div class="form-group">
    <div class="checkbox-group">
        <input type="checkbox" id="passwordProtected" onchange="togglePasswordField()">
        <label for="passwordProtected">设置密码保护</label>
    </div>
</div>

<div class="form-group" id="passwordField" style="display:none;">
    <label for="videoPassword">视频密码</label>
    <input type="password" id="videoPassword" placeholder="输入密码...">
</div>
```

#### c) 字幕文件上传
```html
<!-- 在视频文件上传区域后添加 -->
<div class="form-group">
    <label for="subtitleFile">字幕文件 (可选)</label>
    <input type="file" id="subtitleFile" accept=".srt,.vtt,.txt">
    <p style="color: #999; font-size: 13px; margin-top: 5px;">
        支持 SRT, VTT 格式
    </p>
</div>
```

#### d) JavaScript 更新
```javascript
// 添加语言切换
const translations = {
    zh: {
        title: "视频分享平台",
        upload: "上传视频",
        // ... 其他文本
    },
    en: {
        title: "Video Sharing Platform",
        upload: "Upload Video",
        // ... 其他文本
    }
};

function switchLang(lang) {
    // 更新页面文本
    document.querySelectorAll('[data-i18n]').forEach(el => {
        el.textContent = translations[lang][el.dataset.i18n];
    });
}

// 密码字段切换
function togglePasswordField() {
    const checked = document.getElementById('passwordProtected').checked;
    document.getElementById('passwordField').style.display = checked ? 'block' : 'none';
}

// 表单提交时包含密码和字幕
formData.append('password_protected', document.getElementById('passwordProtected').checked);
formData.append('password', document.getElementById('videoPassword').value);

const subtitleFile = document.getElementById('subtitleFile').files[0];
if (subtitleFile) {
    formData.append('subtitle', subtitleFile);
}
```

### 2. 后台管理页改进 (`web/templates/admin/dashboard.html`)

#### a) 视频列表添加字幕状态和操作
```html
<!-- 在操作列添加字幕按钮 -->
<td>
    <div class="actions">
        <button class="btn btn-primary btn-small" onclick="window.open('/v/{{ .Slug }}', '_blank')">播放</button>
        <button class="btn btn-secondary btn-small" onclick="editVideo({{ .ID }})">编辑</button>

        <!-- 新增字幕按钮 -->
        {{ if .SubtitlePath }}
        <span class="badge badge-success">📄 有字幕</span>
        {{ else }}
        <button class="btn btn-warning btn-small" onclick="generateSubtitle({{ .ID }})">生成字幕</button>
        {{ end }}

        <button class="btn btn-danger btn-small" onclick="deleteVideo({{ .ID }}, '{{ .Title }}')">删除</button>
    </div>
</td>
```

#### b) JavaScript 添加字幕生成功能
```javascript
async function generateSubtitle(videoId) {
    if (!confirm('生成字幕需要调用 OpenAI API，可能需要几分钟。确定继续吗？')) return;

    try {
        const response = await fetch(`/api/admin/videos/${videoId}/generate-subtitle`, {
            method: 'POST'
        });
        const result = await response.json();

        if (response.ok) {
            alert(result.message || '字幕生成已开始，请稍后刷新页面查看');
        } else {
            alert('生成失败: ' + (result.error || '未知错误'));
        }
    } catch (error) {
        alert('网络错误: ' + error.message);
    }
}
```

#### c) 编辑页面添加字幕上传
```html
<!-- 在editForm中添加 -->
<div class="form-group">
    <label>上传字幕文件</label>
    <input type="file" id="editSubtitleFile" accept=".srt,.vtt">
</div>

<!-- JavaScript -->
async function uploadSubtitle(videoId, file) {
    const formData = new FormData();
    formData.append('subtitle', file);

    const response = await fetch(`/api/admin/videos/${videoId}/upload-subtitle`, {
        method: 'POST',
        body: formData
    });

    return response.json();
}
```

### 3. 播放器字幕支持 (`web/templates/watch.html`)

#### a) 添加字幕加载
```html
<video id="video" controls>
    {{ if .video.SubtitlePath }}
    <track kind="subtitles"
           src="/subtitles/{{ .video.Slug }}.vtt"
           srclang="zh"
           label="中文"
           default>
    {{ end }}
</video>
```

#### b) 字幕样式 CSS
```html
<style>
    video::cue {
        font-size: {{ .subtitleConfig.font_size }};
        background: {{ .subtitleConfig.background }};
    }

    /* 中文字幕样式 */
    video::cue(v.chinese) {
        color: {{ .subtitleConfig.chinese_color }};
        font-family: {{ .subtitleConfig.chinese_font }};
    }

    /* 原文字幕样式 */
    video::cue(v.original) {
        color: {{ .subtitleConfig.original_color }};
        font-family: {{ .subtitleConfig.original_font }};
    }
</style>
```

#### c) 添加字幕路由
在 `cmd/server/main.go` 中添加：
```go
// 添加字幕文件服务
r.GET("/subtitles/:filename", func(c *gin.Context) {
    filename := c.Param("filename")
    subtitlePath := filepath.Join(cfg.Storage.SubtitlesDir, filename)
    c.File(subtitlePath)
})
```

### 4. 上传处理器更新 (`internal/handler/upload.go`)

需要在 `UploadVideo` 函数中添加密码和字幕处理：

```go
// 获取密码保护选项
passwordProtected := c.PostForm("password_protected") == "true"
password := c.PostForm("password")

// 处理字幕文件
subtitleFile, _ := c.FormFile("subtitle")
var subtitlePath string
if subtitleFile != nil {
    // 保存字幕文件
    subtitleService := service.NewSubtitleService(h.config)
    content := /* 读取字幕文件内容 */
    subtitlePath, _ = subtitleService.SaveUploadedSubtitle(video.Slug, content)
}

// 在CreateVideo调用中传递这些参数
video, err := h.videoService.CreateVideo(
    savePath,
    file.Filename,
    title,
    description,
    uploadType,
    passwordProtected,
    password,
)

// 如果有字幕，更新视频记录
if subtitlePath != "" {
    database.DB.Model(&video).Update("subtitle_path", subtitlePath)
}
```

---

## 使用说明

### 配置 OpenAI API
编辑 `config.yaml`:
```yaml
openai:
  api_key: "sk-your-api-key-here"
  whisper_model: "whisper-1"
  translation_model: "gpt-4o-mini"
```

### CLI 工具使用
```bash
# 编译
go build -o videocli cmd/videocli/main.go

# 列出视频
./videocli --action list

# 设置密码
./videocli --action set-password --slug abc123 --password mypassword

# 设为公开
./videocli --action set-public --slug abc123
```

### 生成字幕流程
1. 上传视频后，在管理后台点击"生成字幕"按钮
2. 系统调用 Whisper API 转录音频
3. 如果不是中文，自动翻译并生成双语字幕
4. 字幕文件保存为 VTT 格式
5. 播放器自动加载字幕

---

## 注意事项

1. **API 费用**: Whisper 和翻译 API 会产生费用，请注意控制使用
2. **处理时间**: 长视频可能需要几分钟生成字幕
3. **字幕格式**: 推荐使用 VTT 格式，SRT 会自动转换
4. **多语言**: 前端多语言切换需要完整实现 translations 对象

---

## 下一步建议

1. 实现上传页面的完整多语言支持
2. 添加字幕编辑功能
3. 批量生成字幕功能
4. 字幕搜索功能
5. 视频预览生成
