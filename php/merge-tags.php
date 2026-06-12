<?php
if (php_sapi_name() !== 'cli') {
    die("❌ 请在命令行运行\n");
}
require __DIR__ . '/wp-config.php';
global $wpdb, $polylang;
$csv_file = __DIR__ . '/output/tag_mapping_result.csv';
if (!file_exists($csv_file)) {
    die("❌ CSV文件不存在: {$csv_file}\n");
}
// 解析命令行参数
$test_ids = [];
$is_all = false;
$dry_run = in_array('--dry-run', $argv);
if ($dry_run) {
    echo "🏃‍♂️ 【模拟执行模式】：仅输出将要执行的操作，不会修改数据库！\n\n";
}
$argv = array_diff($argv, ['--dry-run']);
if (isset($argv[1])) {
    if ($argv[1] === '--all') {
        $is_all = true;
        echo "🚀 批量模式：处理 CSV 中所有 API匹配成功 的映射\n\n";
    } else {
        $test_ids = array_slice($argv, 1);
        $test_ids = array_map('intval', $test_ids);
        $test_ids = array_filter($test_ids); 
        if (empty($test_ids)) {
            die("❌ 无效的ID参数。用法: php merge-tags.php 7 27 [--dry-run] 或 php merge-tags.php --all [--dry-run]\n");
        }
        echo "🧪 测试模式：仅处理源标签 ID 为 [" . implode(', ', $test_ids) . "] 的映射\n\n";
    }
} else {
    die("ℹ️ 请指定要测试的源标签ID，或使用 --all。\n用法: php merge-tags.php 7 27 [--dry-run]\n用法: php merge-tags.php --all [--dry-run]\n");
}
$target_lang = 'en';
$processed = 0;
/**
 * 穿透式获取英文翻译ID函数
 */
function get_en_term_id_safe($zh_term_id, $target_lang) {
    global $wpdb;
    $en_id = pll_get_term($zh_term_id, $target_lang);
    if ($en_id) return ['id' => $en_id, 'method' => 'Polylang官方函数'];
    $translations = $wpdb->get_var($wpdb->prepare(
        "SELECT meta_value FROM $wpdb->termmeta WHERE term_id = %d AND meta_key = '_pll_translations_post_tag'",
        $zh_term_id
    ));
    if ($translations) {
        $trans = maybe_unserialize($translations);
        if (!empty($trans[$target_lang])) return ['id' => $trans[$target_lang], 'method' => '当前Term的Meta直接提取'];
    }
    $search_string = sprintf('i:%d;', $zh_term_id);
    $meta_values = $wpdb->get_col($wpdb->prepare(
        "SELECT meta_value FROM $wpdb->termmeta WHERE meta_key = '_pll_translations_post_tag' AND meta_value LIKE %s",
        '%' . $wpdb->esc_like($search_string) . '%'
    ));
    if (!empty($meta_values)) {
        foreach ($meta_values as $meta_value) {
            $trans = maybe_unserialize($meta_value);
            if (!empty($trans[$target_lang])) return ['id' => $trans[$target_lang], 'method' => '逆向Meta查找'];
        }
    }
    $zh_term = get_term($zh_term_id, 'post_tag');
    if ($zh_term && !is_wp_error($zh_term)) {
        $en_terms = get_terms([
            'taxonomy' => 'post_tag',
            'slug' => $zh_term->slug,
            'lang' => $target_lang,
            'hide_empty' => false,
        ]);
        if (!is_wp_error($en_terms) && !empty($en_terms)) return ['id' => $en_terms[0]->term_id, 'method' => '按别名+语言查找'];
    }
    return ['id' => 0, 'method' => '未找到'];
}
if (($handle = fopen($csv_file, 'r')) !== FALSE) {
    fgetcsv($handle); // 跳过表头
    while (($data = fgetcsv($handle)) !== FALSE) {
        if (count($data) < 5) continue;
        $source_zh_id = intval($data[0]);
        $source_zh_name = trim($data[1]);
        $target_zh_id = intval($data[2]);
        $target_zh_name = trim($data[3]);
        $status = trim($data[4]);
        if ($status !== 'API匹配成功' || empty($target_zh_id)) continue;
        if (!$is_all && !in_array($source_zh_id, $test_ids)) continue;
        echo "🔄 ========================================\n";
        echo "🔄 准备处理: [中] {$source_zh_name} (ID: {$source_zh_id}) -> [中] {$target_zh_name} (ID: {$target_zh_id})\n";
        // ==============================================================================
        // 核心逻辑：严格校验，绝不自动创建
        // ==============================================================================
        $source_en_info = get_en_term_id_safe($source_zh_id, $target_lang);
        $target_en_info = get_en_term_id_safe($target_zh_id, $target_lang);
        $source_en_id = $source_en_info['id'];
        $target_en_id = $target_en_info['id'];
        echo "  🔍 源英文关联查询: ID=" . ($source_en_id ?: '无') . " (通过" . $source_en_info['method'] . "获取)\n";
        echo "  🔍 目标英文关联查询: ID=" . ($target_en_id ?: '无') . " (通过" . $target_en_info['method'] . "获取)\n";
        $can_merge_en = false;
        if ($source_en_id) {
            // 源标签有英文版，目标英文也必须已存在
            if ($target_en_id) {
                $can_merge_en = true; 
            } else {
                echo "  ❌ 致命错误：源标签有英文(ID:{$source_en_id})，但目标标签无英文！为保证数据一致，跳过本次中英文合并！\n";
                continue; // 直接中止整行操作，中文也不合并
            }
        } else {
            echo "  ℹ️ 源标签无英文翻译，仅合并中文即可\n";
        }
        // ==============================================================================
        // 执行阶段：条件已全部满足，开始合并
        // ==============================================================================
        $zh_success = false;
        // 1. 合并中文
        if ($dry_run) {
            echo "  [模拟] 将删除中文源标签 {$source_zh_id}，并将文章转移至 {$target_zh_id}\n";
            $zh_success = true; 
        } else {
            $result_zh = wp_delete_term($source_zh_id, 'post_tag', ['default' => $target_zh_id, 'force_default' => true]);
            if (is_wp_error($result_zh)) {
                echo "  ❌ 中文合并失败: " . $result_zh->get_error_message() . " (英文也不再执行合并)\n";
                continue; 
            }
            echo "  ✅ 中文合并成功！\n";
            $zh_success = true;
        }
        // 2. 合并英文 (只有当中文成功，且校验允许合并英文时才执行)
        if ($zh_success && $can_merge_en) {
            echo "🔄 准备处理: [英] {$source_zh_name} (ID: {$source_en_id}) -> [英] {$target_zh_name} (ID: {$target_en_id})\n";
            if ($dry_run) {
                echo "  [模拟] 将删除英文源标签 {$source_en_id}，并将文章转移至 {$target_en_id}\n";
            } else {
                $result_en = wp_delete_term($source_en_id, 'post_tag', ['default' => $target_en_id, 'force_default' => true]);
                if (is_wp_error($result_en)) {
                    echo "  ❌ 英文合并失败: " . $result_en->get_error_message() . "\n";
                } else {
                    echo "  ✅ 英文合并成功！\n";
                }
            }
        }
        $processed++;
    }
    fclose($handle);
}
echo "\n🎉 执行完毕！\n";
echo "📊 本次共" . ($dry_run ? '模拟' : '') . "成功合并: {$processed} 组映射\n";
if ($dry_run) {
    echo "💡 提示：去掉 --dry-run 参数后才会真正写入数据库。\n";
}
