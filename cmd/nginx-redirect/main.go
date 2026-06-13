package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// MergeLogRecord 定义合并日志记录结构
type MergeLogRecord struct {
	SourceLang  string `json:"source_lang"`
	SourceID    int    `json:"source_id"`
	SourceName  string `json:"source_name"`
	TargetLang  string `json:"target_lang"`
	TargetID    int    `json:"target_id"`
	TargetName  string `json:"target_name"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
}

// FixEnTagsLogRecord 定义英文标签修复日志记录结构
type FixEnTagsLogRecord struct {
	TermID        int    `json:"term_id"`
	OriginalName  string `json:"original_name"`
	NewName       string `json:"new_name"`
	OldSlug       string `json:"old_slug"`
	NewSlug       string `json:"new_slug"`
	Lang          string `json:"lang"`
	Status        string `json:"status"`
	Timestamp     string `json:"timestamp"`
}

func main() {
	// 1. 读取全量 Slug 字典
	slugMap := make(map[string]string)
	loadAllSlugs("../../data/all_terms_slug.csv", slugMap)

	// 2. 生成 Nginx 规则
	var nginxConf strings.Builder
	nginxConf.WriteString("map $uri $new_tag_uri {\n")
	nginxConf.WriteString("    default \"\";\n")

	// 3. 处理 merge_log.json（标签合并日志）
	processMergeLog("../../output/merge_log.json", slugMap, &nginxConf)

	// 4. 处理 fix_en_tags_log.json（英文标签修复日志）
	processFixEnTagsLog("../../output/fix_en_tags_log.json", &nginxConf)

	nginxConf.WriteString("}\n\n")

	// 调整位置与缩进：将 include 注释放在 server 块前面，与 server 同级顶格对齐
	nginxConf.WriteString("# 引入生成的 map 规则\n")
	nginxConf.WriteString("# include /path/to/nginx_redirect.conf;\n\n")
	nginxConf.WriteString("server {\n")
	nginxConf.WriteString("    # ... 你的其他配置\n\n")
	nginxConf.WriteString("    if ($new_tag_uri != \"\") {\n")
	nginxConf.WriteString("        return 301 $new_tag_uri;\n")
	nginxConf.WriteString("    }\n")
	nginxConf.WriteString("}\n")

	// 5. 写入文件
	outputDir := "../../output"
	outputFile := outputDir + "/nginx_redirect.conf"

	// 如果目录不存在则自动创建
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		fmt.Printf("❌ 创建输出目录失败: %v\n", err)
		return
	}

	err := os.WriteFile(outputFile, []byte(nginxConf.String()), 0644)
	if err != nil {
		fmt.Printf("❌ 写入 Nginx 规则文件失败: %v\n", err)
		return
	}

	fmt.Println("✅ Nginx 规则已成功生成至 ../../output/nginx_redirect.conf")
}

// processMergeLog 处理标签合并日志
func processMergeLog(logFile string, slugMap map[string]string, nginxConf *strings.Builder) {
	logData, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Printf("⚠️ 未找到合并日志文件 %s: %v\n", logFile, err)
		return
	}

	var mergeLog []MergeLogRecord
	if err := json.Unmarshal(logData, &mergeLog); err != nil {
		fmt.Printf("❌ 解析合并日志 JSON 失败: %v\n", err)
		return
	}

	fmt.Printf("📝 从 %s 读取 %d 条合并记录\n", logFile, len(mergeLog))

	for _, record := range mergeLog {
		if record.Status != "success" {
			continue
		}

		if record.SourceLang != record.TargetLang {
			fmt.Printf("⚠️ 警告：语言不一致，跳过。源: %s, 目标: %s\n", record.SourceLang, record.TargetLang)
			continue
		}

		prefix := "/tag/"
		if record.SourceLang == "英" {
			prefix = "/en/tag/"
		}

		srcSlug := slugMap[fmt.Sprintf("%d", record.SourceID)]
		dstSlug := slugMap[fmt.Sprintf("%d", record.TargetID)]

		if srcSlug == "" || dstSlug == "" {
			fmt.Printf("⚠️ 警告：字典中未找到 Slug 映射，跳过。源ID: %d, 目标ID: %d\n", record.SourceID, record.TargetID)
			continue
		}

		if srcSlug == dstSlug {
			continue
		}

		rule := fmt.Sprintf("    ~^%s%s/?$ \"%s%s/\";\n", prefix, srcSlug, prefix, dstSlug)
		nginxConf.WriteString(rule)
	}
}

// processFixEnTagsLog 处理英文标签修复日志
func processFixEnTagsLog(logFile string, nginxConf *strings.Builder) {
	logData, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Printf("⚠️ 未找到英文标签修复日志文件 %s: %v\n", logFile, err)
		return
	}

	var fixLog []FixEnTagsLogRecord
	if err := json.Unmarshal(logData, &fixLog); err != nil {
		fmt.Printf("❌ 解析英文标签修复日志 JSON 失败: %v\n", err)
		return
	}

	fmt.Printf("📝 从 %s 读取 %d 条英文标签修复记录\n", logFile, len(fixLog))

	for _, record := range fixLog {
		if record.Status != "success" {
			continue
		}

		// 英文标签修复只处理 English 语言
		prefix := "/en/tag/"

		oldSlug := record.OldSlug
		newSlug := record.NewSlug

		if oldSlug == "" || newSlug == "" {
			fmt.Printf("⚠️ 警告：Slug 为空，跳过。TermID: %d\n", record.TermID)
			continue
		}

		if oldSlug == newSlug {
			continue
		}

		rule := fmt.Sprintf("    ~^%s%s/?$ \"%s%s/\";\n", prefix, oldSlug, prefix, newSlug)
		nginxConf.WriteString(rule)
	}
}

// loadAllSlugs 读取全量字典
func loadAllSlugs(filePath string, slugMap map[string]string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("❌ 无法打开字典文件 %s: %v\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()

	for i, row := range records {
		if i == 0 {
			continue // 跳过表头
		}
		if len(row) >= 2 {
			termID := strings.Trim(row[0], "\"")
			slug := strings.Trim(row[1], "\"")

			// 将 URL 编码的 Slug (如 %e6%9d%83%e9%99%90) 转换为中文 (如 权限)
			decodedSlug, err := url.PathUnescape(slug)
			if err == nil {
				slug = decodedSlug
			}
			slugMap[termID] = slug
		}
	}
}
