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
	ZhCSVPath     = "data/zh_tags_containing_chinese.csv"
	EnCSVPath     = "data/zh_tags_without_chinese.csv"
	OutCSVPath    = "output/tag_mapping_result.csv"
	CachePath     = "output/translation_cache.json"
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

// 翻译缓存结构
type TranslationCache struct {
	Version    string            `json:"version"`
	CacheTime  string            `json:"cache_time"`
	Translations map[string]string `json:"translations"`
}

// 全局变量存储 API 凭证
var (
	baiduAppID     string
	baiduSecretKey string
	transCache     map[string]string
)

func main() {
	// 1. 加载环境变量 (优先读取 docker-compose 注入的，如果没有则尝试读取 .env 文件)
	_ = godotenv.Load()
	baiduAppID = os.Getenv("BAIDU_APP_ID")
	baiduSecretKey = os.Getenv("BAIDU_SECRET_KEY")

	// 2. 加载翻译缓存（即使没有 API 凭证也可以加载缓存）
	loadTranslationCache()

	// 3. 确保输出目录存在
	os.MkdirAll("output", os.ModePerm)

	// 4. 读取并构建英文标签索引
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

	// 5. 读取中文标签
	zhFile, err := os.Open(ZhCSVPath)
	if err != nil {
		log.Fatalf("无法打开 %s: %v", ZhCSVPath, err)
	}
	defer zhFile.Close()

	zhReader := csv.NewReader(zhFile)
	zhReader.Read() // 跳过表头

	// 6. 准备输出文件
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
	cacheHitCount := 0
	apiCallCount := 0

	// 7. 遍历中文标签进行翻译碰撞
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

		// 尝试从缓存获取翻译
		var translatedName string
		var useCache bool
		var errMsg error

		if cached, ok := transCache[zhName]; ok {
			translatedName = cached
			useCache = true
			cacheHitCount++
			log.Printf("📋 [缓存命中] ID:%s Name:%s -> %s", zhTermID, zhName, translatedName)
		} else {
			// 需要调用 API
			if baiduAppID == "" || baiduSecretKey == "" {
				log.Printf("❌ [跳过] ID:%s Name:%s, 无API凭证且缓存未命中", zhTermID, zhName)
				outWriter.Write([]string{zhTermID, zhName, "", "", "无API凭证(缓存未命中)"})
				continue
			}

			translatedName, errMsg = translateWithBaidu(zhName)
			useCache = false
			apiCallCount++

			if errMsg != nil {
				log.Printf("❌ [翻译失败] ID:%s Name:%s, Error:%v", zhTermID, zhName, errMsg)
				outWriter.Write([]string{zhTermID, zhName, "", "", "翻译失败"})
				time.Sleep(SleepInterval)
				continue
			}

			// 缓存新翻译结果
			transCache[zhName] = translatedName
		}

		searchKey := strings.ToLower(strings.TrimSpace(translatedName))
		if enTag, exists := enTagsMap[searchKey]; exists {
			status := "API匹配成功"
			if useCache {
				status = "缓存匹配成功"
			}
			outWriter.Write([]string{zhTermID, zhName, strconv.Itoa(enTag.TermID), enTag.Name, status})
			matchedCount++
		} else {
			status := "API未匹配(建议Slug)"
			if useCache {
				status = "缓存未匹配(建议Slug)"
			}
			outWriter.Write([]string{zhTermID, zhName, "", translatedName, status})
		}

		if !useCache {
			time.Sleep(SleepInterval)
		}

		if count%50 == 0 {
			log.Printf("⏳ 进度：已处理 %d 条，缓存命中 %d 条，API调用 %d 次，匹配成功 %d 条...",
				count, cacheHitCount, apiCallCount, matchedCount)
		}
	}

	// 8. 保存更新后的缓存
	saveTranslationCache()

	log.Printf("🎉 处理完成！共处理 %d 条，缓存命中 %d 条，API调用 %d 次，成功碰撞 %d 条。结果已保存至 %s",
		count, cacheHitCount, apiCallCount, matchedCount, OutCSVPath)
}

// loadTranslationCache 加载翻译缓存
func loadTranslationCache() {
	transCache = make(map[string]string)

	if _, err := os.Stat(CachePath); os.IsNotExist(err) {
		log.Printf("ℹ️ 未找到翻译缓存文件，将创建新缓存。")
		return
	}

	data, err := os.ReadFile(CachePath)
	if err != nil {
		log.Printf("⚠️ 读取缓存文件失败: %v，将创建新缓存。", err)
		return
	}

	var cache TranslationCache
	if err := json.Unmarshal(data, &cache); err != nil {
		log.Printf("⚠️ 解析缓存文件失败: %v，将创建新缓存。", err)
		return
	}

	transCache = cache.Translations
	log.Printf("✅ 成功加载翻译缓存，共 %d 条记录，缓存时间: %s", len(transCache), cache.CacheTime)
}

// saveTranslationCache 保存翻译缓存
func saveTranslationCache() {
	cache := TranslationCache{
		Version:    "1.0",
		CacheTime:  time.Now().Format("2006-01-02 15:04:05"),
		Translations: transCache,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		log.Printf("⚠️ 保存缓存失败: %v", err)
		return
	}

	if err := os.WriteFile(CachePath, data, 0644); err != nil {
		log.Printf("⚠️ 写入缓存文件失败: %v", err)
		return
	}

	log.Printf("✅ 翻译缓存已保存至 %s，共 %d 条记录", CachePath, len(transCache))
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