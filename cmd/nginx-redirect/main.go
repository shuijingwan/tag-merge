package main
import (
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)
func main() {
	// 1. 读取全量 Slug 字典
	slugMap := make(map[string]string)
	loadAllSlugs("../../data/all_terms_slug.csv", slugMap)
	// 2. 读取日志文件
	logData, err := os.ReadFile("../../output/merge_log.txt")
	if err != nil {
		fmt.Printf("❌ 读取日志文件失败: %v\n", err)
		return
	}
	logStr := string(logData)
	// 正则匹配：准备处理: [中/英] xxx (ID: 27) -> [中/英] yyy (ID: 1519)
	re := regexp.MustCompile(`准备处理: \[(中|英)\] .+ \(ID: (\d+)\) -> \[(中|英)\] .+ \(ID: (\d+)\)`)
	matches := re.FindAllStringSubmatch(logStr, -1)
	// 3. 生成 Nginx 规则
	var nginxConf strings.Builder
	nginxConf.WriteString("map $uri $new_tag_uri {\n")
	nginxConf.WriteString("    default \"\";\n")
	for _, match := range matches {
		srcLang := match[1] // 中 或 英
		srcID := match[2]
		dstLang := match[3] // 中 或 英
		dstID := match[4]
		// 理论上源和目标语言一致，若不一致跳过
		if srcLang != dstLang {
			continue
		}
		prefix := "/tag/"
		if srcLang == "英" {
			prefix = "/en/tag/"
		}
		srcSlug := slugMap[srcID]
		dstSlug := slugMap[dstID]
		if srcSlug == "" || dstSlug == "" {
			fmt.Printf("⚠️ 警告：字典中未找到 Slug 映射，跳过。源ID: %s, 目标ID: %s\n", srcID, dstID)
			continue
		}
		// 如果源和目标 Slug 一样，说明不需要跳转
		if srcSlug == dstSlug {
			continue
		}
		// 生成规则 (将 /$ 改为 /?$，匹配尾部有无斜杠的情况)
		rule := fmt.Sprintf("    ~^%s%s/?$ \"%s%s/\";\n", prefix, srcSlug, prefix, dstSlug)
		nginxConf.WriteString(rule)
	}
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
	// 4. 写入文件
	outputDir := "../../output"
	outputFile := outputDir + "/nginx_redirect.conf"
	// 如果目录不存在则自动创建
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		fmt.Printf("❌ 创建输出目录失败: %v\n", err)
		return
	}
	err = os.WriteFile(outputFile, []byte(nginxConf.String()), 0644)
	if err != nil {
		fmt.Printf("❌ 写入 Nginx 规则文件失败: %v\n", err)
		return
	}
	fmt.Println("✅ Nginx 规则已成功生成至 ../../output/nginx_redirect.conf")
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