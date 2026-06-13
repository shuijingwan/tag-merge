# Tag Merge 🏷️🔄

一个跨环境的 WordPress Polylang 标签处理工具集。涵盖标签同步、翻译、合并与 Nginx 跳转规则生成，形成完整的标签生命周期处理闭环。

专为解决 WordPress Polylang 插件中大量未映射的中英标签而设计，通过本地计算与线上运行时协作，高效完成数万级标签的整理与 SEO 优化。

## 📚 使用场景与背景

想要了解本工具集产生的具体业务场景、踩坑经历以及完整的实操演示，欢迎阅读博客系列文章：
👉 **[WordPress 标签清理实践（一）（二）（三）](https://www.shuijingwanwq.com/2026/06/12/16986/)**

## 🔄 核心工作流与运行环境

本工具集包含 4 个核心步骤，跨越本地和线上环境：

| 步骤       | 脚本                                   | 运行环境      | 说明                                                                                               |
| :--------- | :------------------------------------- | :------------ | :------------------------------------------------------------------------------------------------- |
| **Step 1** | `php/polylang-batch-zh-to-en-tags.php` | ☁️ 线上服务器 | 依赖 WP/Polylang 函数，将中文（中国）语言下的标签批量同步至 English 语言下（生成未翻译的占位标签） |
| **Step 2** | `main.go`                              | 🖥️ 本地       | 导出线上标签为 CSV，调用百度 API 翻译并智能碰撞，生成映射关系 CSV                                  |
| **Step 3** | `php/merge-tags.php`                   | ☁️ 线上服务器 | 依赖 WP/Polylang 函数，读取映射 CSV，将中文标签合并至正确的英文标签并建立关联                      |
| **Step 4** | `cmd/nginx-redirect/main.go`           | 🖥️ 本地       | 读取合并日志与全量 Slug 字典，生成 Nginx 301 跳转配置文件，恢复旧标签的 SEO 权重                   |

**为什么跨环境？**

- **线上 PHP**：涉及 WordPress 数据库操作与 Polylang 钩子，必须在现有 Web 服务器运行时中执行。
- **本地 Go**：负责重计算（翻译匹配）和配置生成，使用 Docker 隔离环境确保一致性且不消耗线上资源。

## ✨ 功能特性

- **批量同步**：一键将中文（中国）语言下的标签批量同步至 English 语言下，解决需要手动添加 Polylang 下 English 语言下的对应标签的问题，提供占位。
- **自动翻译**：接入百度通用文本翻译 API，批量处理中文标签。
- **智能碰撞**：将翻译结果与已有纯英文标签库进行全匹配，自动建立映射关系。
- **安全合并**：在线上环境通过 WP 官方钩子合并标签，避免直接操作数据库带来的风险，支持安全模拟运行。
- **SEO 恢复**：自动推导 Slug 变化并生成 Nginx 301 跳转规则，将旧标签 URL 权重无缝转移至新标签。
- **容器化运行**：本地 Go 环境基于 Docker Compose 构建，开箱即用，不污染宿主机。
- **安全合规**：API 密钥通过 `.env` 管理，杜绝敏感信息泄露至 Git 仓库。

## 📂 目录结构

```text
tag-merge/
├── .devcontainer/          # 容器开发环境配置（可选）
│   └── devcontainer.json
├── cmd/                    # 本地执行的辅助 Go 命令
│   └── nginx-redirect/     # Nginx 301 跳转规则生成器
│       └── main.go
├── data/                   # 存放从数据库导出的原始 CSV 文件
│   ├── zh_tags_containing_chinese.csv  # [Step 2 依赖] 中文（中国）语言下包含中文的标签
│   ├── zh_tags_without_chinese.csv     # [Step 2 依赖] 中文（中国）语言下不包含中文的标签
│   └── all_terms_slug.csv  # [Step 4 依赖] 全站标签 ID-Slug 字典
├── output/                 # 存放程序运行后生成的结果文件
│   ├── tag_mapping_result.csv  # 翻译碰撞结果
│   ├── translation_cache.json  # 翻译缓存（自动生成，复用翻译结果）
│   ├── merge_log.json          # [Step 4 依赖] 线上合并日志（JSON 格式，自动生成）
│   └── nginx_redirect.conf     # 生成的 Nginx 跳转规则
├── php/                    # 线上执行脚本（需上传至线上 Web 服务器运行）
│   ├── polylang-batch-zh-to-en-tags.php  # Step 1
│   └── merge-tags.php                   # Step 3
├── .env.example            # 环境变量示例文件
├── .env                    # 环境变量真实文件（已被 .gitignore 忽略）
├── .gitignore
├── docker-compose.yml      # 本地 Go 开发环境配置
├── Dockerfile              # 本地 Go 开发环境配置
├── go.mod
├── main.go                 # Step 2: 核心翻译碰撞逻辑
└── README.md
```

## 🛠️ 前置要求与数据准备

在执行工作流之前，请确保准备好以下环境和数据：

1. **基础环境**：已安装 Docker 和 Docker Compose (V2)。
2. **API 凭证**：拥有百度翻译开放平台账号，并开通了**通用文本翻译** API，获取了 `APP ID` 和 `密钥`。
3. **导出数据 CSV**：从线上 WordPress 数据库执行以下 SQL，将结果导出为 CSV 文件放入本地 `data/` 目录。
    - **`data/zh_tags_containing_chinese.csv`** (供 Step 2 翻译碰撞使用：中文（中国）语言下包含中文的标签)

        ```sql
        SELECT
          t.term_id,
          t.name,
          t.slug
        FROM
          wp_terms t
          INNER JOIN wp_term_taxonomy tt ON t.term_id = tt.term_id
          INNER JOIN wp_term_relationships tr ON tr.object_id = t.term_id
          INNER JOIN wp_term_taxonomy tt_lang ON tt_lang.term_taxonomy_id = tr.term_taxonomy_id
          INNER JOIN wp_terms t_lang ON t_lang.term_id = tt_lang.term_id
        WHERE
          tt.taxonomy = 'post_tag'
          AND t.slug REGEXP '[^a-zA-Z0-9_-]'
          AND tt_lang.taxonomy = 'term_language'
          AND t_lang.slug = 'pll_zh'
        ```

    - **`data/zh_tags_without_chinese.csv`** (供 Step 2 翻译碰撞使用：中文（中国）语言下不包含中文的标签)

        _(注意：此 SQL 查询的是中文（中国）语言下，但名称本身不包含中文字符的标签，如原本就是英文的标签)_

        ```sql
        SELECT
          t.term_id,
          t.name,
          t.slug
        FROM
          wp_terms t
          INNER JOIN wp_term_taxonomy tt ON t.term_id = tt.term_id
          INNER JOIN wp_term_relationships tr ON tr.object_id = t.term_id
          INNER JOIN wp_term_taxonomy tt_lang ON tt_lang.term_taxonomy_id = tr.term_taxonomy_id
          INNER JOIN wp_terms t_lang ON t_lang.term_id = tt_lang.term_id
        WHERE
          tt.taxonomy = 'post_tag'
          AND t.slug REGEXP '^[a-zA-Z0-9_-]+$'
          AND tt_lang.taxonomy = 'term_language'
          AND t_lang.slug = 'pll_zh';
        ```

    - **`data/all_terms_slug.csv`** (供 Step 4 生成 Nginx 规则使用：全站标签 ID-Slug 字典)

        _(此表不过滤语言，包含全站所有标签的 ID 与 Slug 映射)_

        ```sql
        SELECT
          term_id,
          slug
        FROM
          wp_terms
        WHERE
          term_id IN (
            SELECT
              term_id
            FROM
              wp_term_taxonomy
            WHERE
              taxonomy = 'post_tag'
          );
        ```

    **CSV 格式示例：**

    `zh_tags_containing_chinese.csv` & `zh_tags_without_chinese.csv`

    ```csv
    term_id,name,slug
    "7","图片水平垂直居中","%e5%9b%be%e7%89%87%e6%b0%b4%e5%b9%b3%e5%9e%82%e7%9b%b4%e5%b1%85%e4%b8%ad"
    "27","权限","%e6%9d%83%e9%99%90"
    "1519","permission","permission"
    ```

    `all_terms_slug.csv`

    ```csv
    term_id,slug
    "27","%e6%9d%83%e9%99%90"
    "1519","permission"
    "32800","%e6%9d%83%e9%99%90"
    "25672","permission"
    ```

## 🚀 快速开始（核心工作流）

### Step 1: 线上同步标签 (☁️ 线上环境)

将 `php/polylang-batch-zh-to-en-tags.php` 上传至 WordPress 根目录，通过 SSH 执行：

```bash
php polylang-batch-zh-to-en-tags.php
```

_此步骤会将所有中文（中国）语言下的标签复制到 English 语言下作为占位，为后续标签合并提供基础数据。_

### Step 2: 本地翻译碰撞 (🖥️ 本地环境)

**1. 配置环境变量**：

```bash
cp .env.example .env
# 编辑 .env 填入百度翻译 APP_ID 和 SECRET_KEY
```

**2. 启动容器并运行**：

```bash
# 构建并后台启动容器
docker compose up -d --build

# 进入容器终端
docker exec -it tag-merge sh

# 在容器内初始化依赖（仅首次运行需要）
go mod tidy

# 执行翻译碰撞脚本
go run main.go
```

**输出结果说明 (`output/tag_mapping_result.csv`)：**

```csv
源标签ID,源标签名称,目标标签ID,目标标签名称,状态
7,图片水平垂直居中,,"The image is horizontally, vertically, and centered",API未匹配(建议Slug)
27,权限,1519,permission,API匹配成功
20,csdn 改版,,CSDN redesign,API未匹配(建议Slug)
```

- `API匹配成功`：翻译后的英文在库中找到了已有标签，可直接用于 Step 3 合并。
- `缓存匹配成功`：从缓存读取翻译结果并匹配成功（后续执行时出现）。
- `API未匹配(建议Slug)`：库中无此英文标签，但保留了翻译结果，可作新增 Slug 参考。
- `缓存未匹配(建议Slug)`：从缓存读取翻译结果但未匹配到（后续执行时出现）。

**翻译缓存机制**：

脚本会自动生成 `output/translation_cache.json` 缓存文件，记录所有已翻译的结果。后续执行时：

- ✅ 优先使用缓存，跳过重复的 API 调用
- ✅ 缓存命中的翻译结果无需等待 1 秒限频
- ✅ 仅对新增的中文标签调用 API
- ✅ 如果没有 API 凭证但有缓存，可仅使用缓存运行

首次执行可能需要较长时间（受百度 API 1秒1次限频），但第二次及以后执行会非常快！

### Step 3: 线上合并标签 (☁️ 线上环境)

1. 将 Step 2 生成的 `output/tag_mapping_result.csv` 上传至线上服务器。
2. 将 `php/merge-tags.php` 上传至 WordPress 根目录。
3. 执行合并（支持以下四种运行方式，**强烈建议首次运行先使用安全模拟**）：

```bash
# 安全模拟 - 单条（不会修改数据库，仅输出合并预览）
php merge-tags.php 148 --dry-run

# 安全模拟 - 全量（不会修改数据库，仅输出合并预览）
php merge-tags.php --all --dry-run

# 单条实战
php merge-tags.php 148

# 全量实战
php merge-tags.php --all
```

⚠️ **重要**：执行全量实战 `--all` 后，脚本会自动生成 `output/merge_log.json` 文件，供 Step 4 使用。无需手动复制日志！

日志文件格式示例（`merge_log.json`）：

```json
[
    {
        "source_lang": "中",
        "source_id": 27,
        "source_name": "权限",
        "target_lang": "中",
        "target_id": 1519,
        "target_name": "permission",
        "status": "success",
        "timestamp": "2026-06-13 10:30:00"
    },
    {
        "source_lang": "英",
        "source_id": 32800,
        "source_name": "权限",
        "target_lang": "英",
        "target_id": 25672,
        "target_name": "permission",
        "status": "success",
        "timestamp": "2026-06-13 10:30:01"
    }
]
```

终端输出示例（供肉眼观察）：

```text
🔄 ========================================
🔄 准备处理: [中] 权限 (ID: 27) -> [中] permission (ID: 1519)
  🔍 源英文关联查询: ID=32800 (通过Polylang官方函数获取)
  🔍 目标英文关联查询: ID=25672 (通过Polylang官方函数获取)
  ✅ 中文合并成功！
🔄 准备处理: [英] 权限 (ID: 32800) -> [英] permission (ID: 25672)
  ✅ 英文合并成功！

```

### Step 4: 本地生成 Nginx 规则 (🖥️ 本地环境)

在本地 Docker 容器内运行 Go 脚本。此脚本会结合全量字典 `data/all_terms_slug.csv` 和上一步自动生成的合并日志 `output/merge_log.json`，通过 ID 交叉匹配自动推导出 Slug 的变化，并生成 301 跳转规则。

```bash
# 如果在容器内
go run cmd/nginx-redirect/main.go
```

**生成的 Nginx 规则示例 (`output/nginx_redirect.conf`)：**

```nginx
map $uri $new_tag_uri {
    default "";
    ~^/tag/权限/?$ "/tag/permission/";
    ~^/en/tag/权限/?$ "/en/tag/permission/";
}

# 引入生成的 map 规则
# include /path/to/nginx_redirect.conf;

server {
    # ... 你的其他配置

    # 如果 map 匹配到了新的 URI，则执行 301 跳转
    if ($new_tag_uri != "") {
        return 301 $new_tag_uri;
    }
}
```

将生成的配置放入线上 Nginx 并 `nginx -s reload` 即可完成旧链接的 301 跳转。

## ⚠️ 注意事项

- 百度翻译免费版（标准版）有 QPS 限制（1秒1次），`main.go` 中已设置 1.1 秒的请求间隔。如标签量大请耐心等待，或升级高级版修改 `SleepInterval`。
- 线上 PHP 脚本执行时间较长时，注意调整 `php.ini` 中的 `max_execution_time`。

## 📄 License

MIT
