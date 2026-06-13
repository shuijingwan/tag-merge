<?php
if (php_sapi_name() !== 'cli') {
    die("❌ 请在命令行运行\n");
}

require __DIR__ . '/wp-config.php';
global $wpdb, $polylang;

// 解析命令行参数
$dry_run = in_array('--dry-run', $argv);
if ($dry_run) {
    echo "🏃‍♂️ 【模拟执行模式】：仅输出将要执行的操作，不会修改数据库！\n\n";
}

// 配置
$target_lang = 'en'; // 要处理的目标语言

// 📝 日志记录数组（用于生成 JSON 文件）
$fix_log = [];

// 加载翻译缓存
$cache_file = __DIR__ . '/output/translation_cache.json';
if (!file_exists($cache_file)) {
    die("❌ 翻译缓存文件不存在: {$cache_file}\n\n" .
        "💡 提示：请确保 output/translation_cache.json 文件与本脚本平级放置。\n" .
        "   目录结构要求:\n" .
        "   ├── php/\n" .
        "   │   └── fix-en-chinese-tags.php\n" .
        "   └── output/\n" .
        "       └── translation_cache.json\n");
}

$cache_data = json_decode(file_get_contents($cache_file), true);
if (!$cache_data || !isset($cache_data['translations'])) {
    die("❌ 翻译缓存文件格式无效\n");
}

$translations = $cache_data['translations'];
echo "✅ 成功加载翻译缓存，共 " . count($translations) . " 条翻译记录\n\n";

// 查询 English 语言下名称包含中文的标签
$terms = get_terms([
    'taxonomy' => 'post_tag',
    'lang' => $target_lang,
    'hide_empty' => false,
]);

if (is_wp_error($terms) || empty($terms)) {
    die("ℹ️ 未找到 English 语言下的标签\n");
}

// 筛选出名称包含中文的标签
$chinese_terms = [];
foreach ($terms as $term) {
    if (preg_match('/[\x{4e00}-\x{9fff}]/u', $term->name)) {
        $chinese_terms[] = $term;
    }
}

if (empty($chinese_terms)) {
    die("ℹ️ 未找到 English 语言下名称包含中文的标签\n");
}

echo "🔍 发现 " . count($chinese_terms) . " 个 English 语言下名称包含中文的标签：\n";
foreach ($chinese_terms as $term) {
    echo "   - {$term->name} (ID: {$term->term_id}, Slug: {$term->slug})\n";
}
echo "\n";

// 处理每个标签
$processed = 0;
$skipped = 0;
$failed = 0;

foreach ($chinese_terms as $term) {
    $original_name = $term->name;
    $term_id = $term->term_id;
    $old_slug = $term->slug;
    
    // 查找对应的英文翻译
    $english_name = isset($translations[$original_name]) ? $translations[$original_name] : null;
    
    if (!$english_name) {
        echo "❌ 标签 {$original_name} (ID: {$term_id}) 未找到对应的英文翻译，跳过\n";
        $skipped++;
        continue;
    }
    
    // 将翻译转换为合适的 slug 格式（小写、空格转连字符、移除特殊字符）
    $new_slug = strtolower(str_replace(' ', '-', preg_replace('/[^a-zA-Z0-9\s-]/', '', $english_name)));
    
    echo "🔄 准备处理: [{$target_lang}] {$original_name} (ID: {$term_id}) -> {$english_name}\n";
    echo "   └─ Slug: {$old_slug} -> {$new_slug}\n";
    
    if ($dry_run) {
        echo "   [模拟] 跳过实际修改\n";
        $processed++;
        continue;
    }
    
    // 执行修改
    $result = wp_update_term($term_id, 'post_tag', [
        'name' => $english_name,
        'slug' => $new_slug,
    ]);
    
    if (is_wp_error($result)) {
        echo "❌ 修改失败: " . $result->get_error_message() . "\n";
        // 📝 记录失败
        $fix_log[] = [
            'term_id' => $term_id,
            'original_name' => $original_name,
            'new_name' => $english_name,
            'old_slug' => $old_slug,
            'new_slug' => $new_slug,
            'lang' => $target_lang,
            'status' => 'failed',
            'error' => $result->get_error_message(),
            'timestamp' => date('Y-m-d H:i:s')
        ];
        $failed++;
    } else {
        echo "✅ 修改成功！\n";
        // 📝 记录成功
        $fix_log[] = [
            'term_id' => $term_id,
            'original_name' => $original_name,
            'new_name' => $english_name,
            'old_slug' => $old_slug,
            'new_slug' => $new_slug,
            'lang' => $target_lang,
            'status' => 'success',
            'timestamp' => date('Y-m-d H:i:s')
        ];
        $processed++;
    }
}

echo "\n🎉 执行完毕！\n";
echo "📊 统计：\n";
echo "   已处理: {$processed} 个\n";
echo "   跳过(无翻译): {$skipped} 个\n";
echo "   失败: {$failed} 个\n";

// 📝 写入 JSON 日志文件（仅非模拟模式）
if (!$dry_run && !empty($fix_log)) {
    $log_file = __DIR__ . '/output/fix_en_tags_log.json';
    $log_dir = dirname($log_file);
    if (!is_dir($log_dir)) {
        mkdir($log_dir, 0755, true);
    }
    $json_content = json_encode($fix_log, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE);
    if (file_put_contents($log_file, $json_content)) {
        echo "\n📝 日志已保存至: {$log_file}\n";
        echo "💡 此文件供生成 Nginx 301 规则使用\n";
    } else {
        echo "\n❌ 日志文件写入失败\n";
    }
}

if ($dry_run) {
    echo "\n💡 提示：去掉 --dry-run 参数后才会真正修改数据库并生成日志文件。\n";
}
?>