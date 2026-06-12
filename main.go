	package main
	import (
		"crypto/md5"
		"encoding/csv"
		"encoding/hex"
		"encoding/json"
		"fmt"
		"io"
		"log"
		"net/http"
		"net/url"
		"os"
		"strconv"
		"strings"
		"time"
		"github.com/joho/godotenv"
	)
	// ===== 配置项 =====
	const (
		ZhCSVPath     = "data/zh_tags.csv"
		EnCSVPath     = "data/en_tags.csv"
		OutCSVPath    = "output/tag_mapping_result.csv"
		SleepInterval = 1100 * time.Millisecond // 免费版限频 1秒1次
	)
	// 百度翻译返回结构
	type BaiduTransResult struct {
		TransResult []struct {
			Dst string `json:"dst"`
		} `json:"trans_result"`
		ErrorMsg  string `json:"error_msg"`
		ErrorCode int    `json:"error_code"`
	}
	// 标签结构
	type Tag struct {
		TermID int
		Name   string
		Slug   string
	}
	// 全局变量存储 API 凭证
	var (
		baiduAppID     string
		baiduSecretKey string
	)
	func main() {
		// 1. 加载环境变量 (优先读取 docker-compose 注入的，如果没有则尝试读取 .env 文件)
		_ = godotenv.Load()
		baiduAppID = os.Getenv("BAIDU_APP_ID")
		baiduSecretKey = os.Getenv("BAIDU_SECRET_KEY")
		if baiduAppID == "" || baiduSecretKey == "" {
			log.Fatal("错误：未配置百度翻译 API 凭证，请在 docker-compose.yml 或 .env 中设置")
		}
		// 确保输出目录存在
		os.MkdirAll("output", os.ModePerm)
		// 2. 读取并构建英文标签索引
		enTagsMap := make(map[string]Tag)
		enFile, err := os.Open(EnCSVPath)
		if err != nil {
			log.Fatalf("无法打开 %s: %v", EnCSVPath, err)
		}
		defer enFile.Close()
		enReader := csv.NewReader(enFile)
		enReader.Read() // 跳过表头
		for {
			record, err := enReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil || len(record) < 3 {
				continue
			}
			termID, _ := strconv.Atoi(record[0])
			tag := Tag{TermID: termID, Name: record[1], Slug: record[2]}
			enTagsMap[strings.ToLower(tag.Name)] = tag
			enTagsMap[strings.ToLower(tag.Slug)] = tag
		}
		log.Printf("✅ 成功加载英文标签库，共建立 %d 个索引。\n", len(enTagsMap))
		// 3. 读取中文标签
		zhFile, err := os.Open(ZhCSVPath)
		if err != nil {
			log.Fatalf("无法打开 %s: %v", ZhCSVPath, err)
		}
		defer zhFile.Close()
		zhReader := csv.NewReader(zhFile)
		zhReader.Read() // 跳过表头
		// 4. 准备输出文件
		outFile, err := os.Create(OutCSVPath)
		if err != nil {
			log.Fatalf("无法创建输出文件: %v", err)
		}
		defer outFile.Close()
		outFile.Write([]byte{0xEF, 0xBB, 0xBF}) // 写入 UTF-8 BOM，防止 Excel 乱码
		outWriter := csv.NewWriter(outFile)
		defer outWriter.Flush()
		outWriter.Write([]string{"源标签ID", "源标签名称", "目标标签ID", "目标标签名称", "状态"})
		count := 0
		matchedCount := 0
		// 5. 遍历中文标签进行翻译碰撞
		for {
			record, err := zhReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil || len(record) < 2 {
				continue
			}
			zhTermID := record[0]
			zhName := record[1]
			count++
			translatedName, err := translateWithBaidu(zhName)
			if err != nil {
				log.Printf("❌ [翻译失败] ID:%s Name:%s, Error:%v", zhTermID, zhName, err)
				outWriter.Write([]string{zhTermID, zhName, "", "", "翻译失败"})
				time.Sleep(SleepInterval)
				continue
			}
			searchKey := strings.ToLower(strings.TrimSpace(translatedName))
			if enTag, exists := enTagsMap[searchKey]; exists {
				outWriter.Write([]string{zhTermID, zhName, strconv.Itoa(enTag.TermID), enTag.Name, "API匹配成功"})
				matchedCount++
			} else {
				outWriter.Write([]string{zhTermID, zhName, "", translatedName, "API未匹配(建议Slug)"})
			}
			time.Sleep(SleepInterval)
			if count%50 == 0 {
				log.Printf("⏳ 进度：已处理 %d 条，当前匹配成功 %d 条...", count, matchedCount)
			}
		}
		log.Printf("🎉 处理完成！共处理 %d 条，成功碰撞 %d 条。结果已保存至 %s", count, matchedCount, OutCSVPath)
	}
	// translateWithBaidu 调用百度翻译 API
	func translateWithBaidu(query string) (string, error) {
		salt := fmt.Sprintf("%d", time.Now().UnixMilli())
		signStr := baiduAppID + query + salt + baiduSecretKey
		h := md5.New()
		h.Write([]byte(signStr))
		sign := hex.EncodeToString(h.Sum(nil))
		reqURL := fmt.Sprintf("http://api.fanyi.baidu.com/api/trans/vip/translate?q=%s&from=zh&to=en&appid=%s&salt=%s&sign=%s",
			url.QueryEscape(query), baiduAppID, salt, sign)
		resp, err := http.Get(reqURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		var result BaiduTransResult
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("解析JSON失败: %s", string(body))
		}
		if result.ErrorCode != 0 {
			return "", fmt.Errorf("API错误: [%d] %s", result.ErrorCode, result.ErrorMsg)
		}
		if len(result.TransResult) == 0 {
			return "", fmt.Errorf("未获取到翻译结果")
		}
		return result.TransResult[0].Dst, nil
	}