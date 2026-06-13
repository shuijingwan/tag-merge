package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
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

	// 2. 读取现有的 Nginx 配置文件（如果存在），用于去重
	existingRules := loadExistingRules("../../output/nginx_redirect.conf")
	fmt.Printf("📋 已存在 %d 条 Nginx 规则\n", len(existingRules))

	// 3. 生成 Nginx 规则（使用 Set 去重）
	ruleSet := make(map[string]bool)
	for rule := range existingRules {
		ruleSet[rule] = true
	}

	var newRules []string
	var skipCount int

	// 4. 处理 merge_log.json（标签合并日志）
	newRules, skipCount = processMergeLog("../../output/merge_log.json", slugMap, ruleSet)
	fmt.Printf("📝 从 merge_log.json 生成 %d 条新规则，跳过 %d 条重复规则\n", len(newRules), skipCount)

	// 5. 处理 fix_en_tags_log.json（英文标签修复日志）
	moreRules, moreSkipCount := processFixEnTagsLog("../../output/fix_en_tags_log.json", ruleSet)
	fmt.Printf("📝 从 fix_en_tags_log.json 生成 %d 条新规则，跳过 %d 条重复规则\n", len(moreRules), moreSkipCount)
	newRules = append(newRules, moreRules...)

	if len(newRules) == 0 {
		fmt.Println("ℹ️ 没有新规则需要添加（所有规则都已存在）")
		return
	}

	// 6. 插入规则到 map 块内
	outputFile := "../../output/nginx_redirect.conf"
	if err := insertRulesToMapBlock(outputFile, newRules); err != nil {
		fmt.Printf("❌ 插入规则失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 成功插入 %d 条新规则至 %s 的 map 块内\n", len(newRules), outputFile)
	fmt.Println("💡 提示：可以安全地将此文件内容复制到线上 Nginx 配置中，不会重复")
}

// loadExistingRules 读取现有配置文件，提取已有规则
func loadExistingRules(filePath string) map[string]string {
	existingRules := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return existingRules
	}
	defer file.Close()

	// 匹配形如 "    ~^/tag/xxx/?$ "/tag/yyy/";" 的规则
	ruleRegex := regexp.MustCompile(`^\s*~?\^?(/[^ ]+/?\$)\s+"([^"]+)"`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := ruleRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			// 存储 key 为源 URL，value 为目标 URL
			existingRules[matches[1]] = matches[2]
		}
	}

	return existingRules
}

// processMergeLog 处理标签合并日志，返回新规则列表
func processMergeLog(logFile string, slugMap map[string]string, existingRules map[string]bool) ([]string, int) {
	logData, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Printf("⚠️ 未找到合并日志文件 %s: %v\n", logFile, err)
		return []string{}, 0
	}

	var mergeLog []MergeLogRecord
	if err := json.Unmarshal(logData, &mergeLog); err != nil {
		fmt.Printf("❌ 解析合并日志 JSON 失败: %v\n", err)
		return []string{}, 0
	}

	fmt.Printf("📝 从 %s 读取 %d 条合并记录\n", logFile, len(mergeLog))

	var newRules []string
	skipCount := 0

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

		srcPattern := fmt.Sprintf("^%s%s/?$", prefix, srcSlug)
		rule := fmt.Sprintf("    ~^%s%s/?$ \"%s%s/\";", prefix, srcSlug, prefix, dstSlug)

		// 检查是否已存在
		if existingRules[srcPattern] {
			skipCount++
			continue
		}

		newRules = append(newRules, rule)
		existingRules[srcPattern] = true // 标记为已存在
	}

	return newRules, skipCount
}

// processFixEnTagsLog 处理英文标签修复日志，返回新规则列表
func processFixEnTagsLog(logFile string, existingRules map[string]bool) ([]string, int) {
	logData, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Printf("⚠️ 未找到英文标签修复日志文件 %s: %v\n", logFile, err)
		return []string{}, 0
	}

	var fixLog []FixEnTagsLogRecord
	if err := json.Unmarshal(logData, &fixLog); err != nil {
		fmt.Printf("❌ 解析英文标签修复日志 JSON 失败: %v\n", err)
		return []string{}, 0
	}

	fmt.Printf("📝 从 %s 读取 %d 条英文标签修复记录\n", logFile, len(fixLog))

	var newRules []string
	skipCount := 0

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

		srcPattern := fmt.Sprintf("^%s%s/?$", prefix, oldSlug)
		rule := fmt.Sprintf("    ~^%s%s/?$ \"%s%s/\";", prefix, oldSlug, prefix, newSlug)

		// 检查是否已存在
		if existingRules[srcPattern] {
			skipCount++
			continue
		}

		newRules = append(newRules, rule)
		existingRules[srcPattern] = true // 标记为已存在
	}

	return newRules, skipCount
}

// insertRulesToMapBlock 将新规则插入到 map 块内
func insertRulesToMapBlock(filePath string, rules []string) error {
	// 读取现有文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		// 文件不存在，创建新文件
		return createNewConfigFile(filePath, rules)
	}

	lines := strings.Split(string(content), "\n")

	// 找到 map 块的结束位置（第一个单独的 "}"）
	mapBlockEndIndex := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "}" {
			mapBlockEndIndex = i
			break
		}
	}

	if mapBlockEndIndex == -1 {
		// 没有找到 map 块，创建新文件
		return createNewConfigFile(filePath, rules)
	}

	// 在 map 块结束位置之前插入新规则
	newLines := make([]string, 0, len(lines)+len(rules))
	newLines = append(newLines, lines[:mapBlockEndIndex]...)
	for _, rule := range rules {
		newLines = append(newLines, rule)
	}
	newLines = append(newLines, lines[mapBlockEndIndex:]...)

	// 写入文件
	newContent := strings.Join(newLines, "\n")
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// createNewConfigFile 创建新的 Nginx 配置文件
func createNewConfigFile(filePath string, rules []string) error {
	var config strings.Builder

	// 写入 map 块
	config.WriteString("map $uri $new_tag_uri {\n")
	config.WriteString("    default \"\";\n")
	for _, rule := range rules {
		config.WriteString(rule + "\n")
	}
	config.WriteString("}\n\n")

	// 写入 server 块示例
	config.WriteString("# 引入生成的 map 规则\n")
	config.WriteString("# include /path/to/nginx_redirect.conf;\n\n")
	config.WriteString("server {\n")
	config.WriteString("    # ... 你的其他配置\n\n")
	config.WriteString("    if ($new_tag_uri != \"\") {\n")
	config.WriteString("        return 301 $new_tag_uri;\n")
	config.WriteString("    }\n")
	config.WriteString("}\n")

	return os.WriteFile(filePath, []byte(config.String()), 0644)
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