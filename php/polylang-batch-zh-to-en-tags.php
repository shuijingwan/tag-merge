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

echo "🚀 开始批量处理所有{$source_lang}标签，自动添加{$target_lang}翻译（分页模式）...\n\n";

$processed = 0;
$skipped = 0;
$offset = 0;

while (true) {
    // 分页拿源语言标签
    $terms = get_terms([
        'taxonomy' => 'post_tag',
        'lang' => $source_lang,
        'hide_empty' => false,
        'number' => $per_page,
        'offset' => $offset,
    ]);

    if (is_wp_error($terms) || empty($terms)) {
        break;
    }

    $current_count = count($terms);
    echo "📊 正在处理第 " . ($offset + 1) . " - " . ($offset + $current_count) . " 个标签...\n";

    foreach ($terms as $term) {
        $source_id = $term->term_id;
        $name = $term->name;
        $slug = $term->slug;

        // 检查是不是已经有目标语言的翻译了
        $target_id = pll_get_term($source_id, $target_lang);
        if ($target_id) {
            $skipped++;
            continue;
        }

        echo "🔰 正在处理: {$name} (ID: {$source_id})\n";

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
        $polylang->model->term->save_translations($source_id, [
            $target_lang => $new_target_id
        ]);

        $processed++;
    }

    $offset += $per_page;
    unset($terms);
    gc_collect_cycles();
}

echo "\n🎉 全部处理完成！\n";
echo "📊 统计：\n";
echo "   已处理新标签: {$processed}\n";
echo "   已跳过已有翻译: {$skipped}\n";
echo "\n✅ 所有标签都已经处理完毕，和手动添加的完全一样！\n";
