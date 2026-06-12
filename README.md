# Tag Merge 🏷️🔄

一个基于 Go 语言的标签翻译与合并工具。专为解决 WordPress Polylang 插件中大量未映射的中英标签而设计。

通过调用百度翻译 API，将中文标签自动翻译为英文，并与数据库中已有的英文标签进行智能碰撞匹配，最终生成包含映射关系与 Slug 建议的 CSV 文件。

## ✨ 功能特性

- **自动翻译**：接入百度通用文本翻译 API，批量处理中文标签。
- **智能碰撞**：将翻译结果与已有英文标签库（Name/Slug）进行全匹配，自动建立映射关系。
- **Slug 建议**：对未匹配上的标签，保留其英文翻译结果，方便后续直接用作 URL Slug。
- **容器化运行**：基于 Docker Compose 构建，环境隔离，开箱即用，不污染宿主机。
- **安全合规**：API 密钥通过 `.env` 管理，杜绝敏感信息泄露至 Git 仓库。

## 📂 目录结构

```text
tag-merge/
├── .devcontainer/          # 容器开发环境配置（可选）
│   ├── devcontainer.json   # 容器开发环境配置文件
├── data/                   # 存放从数据库导出的原始 CSV 文件
│   ├── zh_tags.csv         # 中文标签库
│   └── en_tags.csv         # 英文标签库
├── output/                 # 存放程序运行后生成的结果文件
│   └── tag_mapping_result.csv
├── .env.example            # 环境变量示例文件
├── .env                    # 环境变量真实文件（已被 .gitignore 忽略）
├── .gitignore
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── main.go                 # 核心业务逻辑
└── README.md
```

## 🚀 快速开始

### 1. 前置要求

- 已安装 Docker 和 Docker Compose (V2)
- 拥有百度翻译开放平台账号，并开通了**通用文本翻译** API，获取了 `APP ID` 和 `密钥`。

### 2. 配置环境变量

复制示例文件并填入您的真实 API 凭证：

```bash
cp .env.example .env
```

编辑 `.env` 文件：
```env
BAIDU_APP_ID=你的真实APPID
BAIDU_SECRET_KEY=你的真实密钥
```

### 3. 准备数据文件

将导出的中文和英文标签 CSV 文件放入 `data/` 目录，确保文件名为 `zh_tags.csv` 和 `en_tags.csv`。

CSV 格式要求（需包含表头）：
```csv
term_id,name,slug
1,开发,development
```

### 4. 启动容器并运行

```bash
# 构建并后台启动容器
docker compose up -d --build

# 进入容器终端
docker exec -it tag-merge sh

# 在容器内初始化依赖（仅首次运行需要）
go mod tidy

# 执行合并脚本
go run main.go
```

## 📊 输出说明

脚本运行完成后，会在 `output/` 目录下生成 `tag_mapping_result.csv`（已加 BOM 头，Excel 打开不会乱码）。

列说明如下：

| 列名 | 说明 |
| :--- | :--- |
| **源标签ID** | 中文标签的 term_id |
| **源标签名称** | 中文标签的 name |
| **目标标签ID** | 匹配成功的英文标签 term_id |
| **目标标签名称** | 匹配成功的英文标签 name |
| **状态** | 见下方状态说明 |

**状态 说明：**
- `API匹配成功`：翻译后的英文在库中找到了已有标签，可直接用于合并。
- `API未匹配(建议Slug)`：库中无此英文标签，但“目标标签名称”列已填入翻译结果，可作为新增标签的 Slug 参考。
- `翻译失败`：API 调用出错（可能是额度超限或网络问题），需检查日志。

## ⚠️ 注意事项

- 百度翻译免费版（标准版）有 QPS 限制（1秒1次），脚本中已设置 1.1 秒的请求间隔。3592 个标签大约需要 65 分钟跑完，请耐心等待。
- 如果您升级了百度翻译高级版，可修改 `main.go` 中的 `SleepInterval` 为更短的时间（如 `100 * time.Millisecond`）。

## 📄 License

MIT
```
