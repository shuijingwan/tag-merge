<?php
if (php_sapi_name() !== 'cli') {
    die("❌ 请在命令行运行\n");
}

require __DIR__ . '/wp-config.php';
global $wpdb, $polylang;

// 配置：一次处理多少个，你可以根据自己的服务器调整
$per_page = 1000; 
// 配置：源语言和目标语言，以后其他语言直接改这里就好
$source_lang = 'zh';
$target_lang = 'en';

echo "🚀 开始批量处理所有{$source_lang}标签，自动添加{$target_lang}翻译...\n\n";

$processed = 0;
$skipped = 0;

// 先一次性获取所有源语言标签的ID列表（使用fields参数避免获取完整对象）
$source_term_ids = get_terms([
    'taxonomy' => 'post_tag',
    'lang' => $source_lang,
    'hide_empty' => false,
    'fields' => 'ids',
]);

if (is_wp_error($source_term_ids)) {
    die("❌ 获取源语言标签失败: " . $source_term_ids->get_error_message() . "\n");
}

$total_terms = count($source_term_ids);
echo "📋 统计信息：源语言({$source_lang})标签总数: {$total_terms}\n\n";

if (empty($source_term_ids)) {
    echo "ℹ️ 没有找到源语言标签，任务结束。\n";
    exit();
}

// 分批处理
$total_batches = ceil($total_terms / $per_page);
$current_batch = 1;

foreach (array_chunk($source_term_ids, $per_page) as $batch) {
    $start_num = ($current_batch - 1) * $per_page + 1;
    $end_num = min($current_batch * $per_page, $total_terms);
    echo "📊 正在处理第 {$start_num} - {$end_num} 个标签（第 {$current_batch}/{$total_batches} 批）...\n";
    
    foreach ($batch as $term_id) {
        $term = get_term($term_id, 'post_tag');
        if (!$term || is_wp_error($term)) {
            continue;
        }
        
        $name = $term->name;
        $slug = $term->slug;

        // 检查是不是已经有目标语言的翻译了
        $target_id = pll_get_term($term_id, $target_lang);
        if ($target_id) {
            $skipped++;
            continue;
        }

        echo "🔰 正在处理: {$name} (ID: {$term_id})\n";

        // 创建目标语言标签，绕过slug重复检查
        $wpdb->insert($wpdb->terms, [
            'name' => $name,
            'slug' => $slug,
            'term_group' => 0
        ]);
        $new_target_id = $wpdb->insert_id;

        // 目标语言标签的taxonomy
        $wpdb->insert($wpdb->term_taxonomy, [
            'term_id'     => $new_target_id,
            'taxonomy'    => 'post_tag',
            'description' => '',
            'parent'      => 0,
            'count'       => 0
        ]);

        // 设置目标语言
        $polylang->model->term->set_language($new_target_id, $target_lang);

        // 绑定翻译
        $polylang->model->term->save_translations($term_id, [
            $target_lang => $new_target_id
        ]);

        $processed++;
    }
    
    $current_batch++;
    unset($batch);
    gc_collect_cycles();
}

echo "\n🎉 全部处理完成！\n";
echo "📊 统计：\n";
echo "   源语言标签总数: {$total_terms}\n";
echo "   已处理新标签: {$processed}\n";
echo "   已跳过已有翻译: {$skipped}\n";

// 验证：检查是否所有源语言标签都有了目标语言翻译
$untranslated_count = 0;
$untranslated_list = [];
foreach ($source_term_ids as $term_id) {
    if (!pll_get_term($term_id, $target_lang)) {
        $untranslated_count++;
        $term = get_term($term_id, 'post_tag');
        if ($term && !is_wp_error($term)) {
            $untranslated_list[] = "{$term->name} (ID: {$term_id})";
        }
    }
}

if ($untranslated_count > 0) {
    echo "\n⚠️ 警告：发现 {$untranslated_count} 个标签未成功添加翻译！\n";
    echo "未翻译的标签：\n";
    foreach ($untranslated_list as $item) {
        echo "  - {$item}\n";
    }
    echo "\n建议重新运行脚本或手动检查这些标签。\n";
} else {
    echo "\n✅ 所有标签都已经处理完毕！\n";
}

echo "\n📈 处理率: " . number_format((($processed + $skipped) / $total_terms) * 100, 2) . "%\n";
