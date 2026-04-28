import {Platform, ScrollView, StyleSheet, View} from 'react-native';
import type {RenderRules} from 'react-native-markdown-display';
import type {AppColors} from './colors';
import {fontSizes} from './typography';

type MarkdownNode = {
  key?: string;
  type?: string;
  children?: MarkdownNode[];
};

type MarkdownTheme = {
  background: string;
  surface: string;
  surfaceSoft: string;
  border: string;
  borderStrong: string;
  text: string;
  textSoft: string;
  heading: string;
  primary: string;
  primarySoft: string;
  quoteBg: string;
  codeBg: string;
  codeText: string;
  tableStripe: string;
};

export const markdownHeadingFontSizes = {
  // 一级标题字号。
  h1: 32,
  // 二级标题字号。
  h2: 24,
  // 三级标题字号。
  h3: 18.72,
  // 四级标题字号。
  h4: 16,
  // 五级标题字号。
  h5: 13.28,
  // 六级标题字号。
  h6: 10.72,
} as const;

function parseHexColor(color: string) {
  const normalized = color.replace('#', '');
  const value =
    normalized.length === 3
      ? normalized
          .split('')
          .map((part) => part + part)
          .join('')
      : normalized;

  return {
    r: parseInt(value.slice(0, 2), 16),
    g: parseInt(value.slice(2, 4), 16),
    b: parseInt(value.slice(4, 6), 16),
  };
}

function rgbToHex({r, g, b}: {r: number; g: number; b: number}) {
  return `#${[r, g, b]
    .map((value) => Math.max(0, Math.min(255, Math.round(value))).toString(16).padStart(2, '0'))
    .join('')}`;
}

function mixColors(base: string, overlay: string, overlayWeight: number) {
  const baseRgb = parseHexColor(base);
  const overlayRgb = parseHexColor(overlay);
  const clampedWeight = Math.max(0, Math.min(1, overlayWeight));

  return rgbToHex({
    r: baseRgb.r * (1 - clampedWeight) + overlayRgb.r * clampedWeight,
    g: baseRgb.g * (1 - clampedWeight) + overlayRgb.g * clampedWeight,
    b: baseRgb.b * (1 - clampedWeight) + overlayRgb.b * clampedWeight,
  });
}

function isDarkTheme(colors: AppColors) {
  return colors.statusBarStyle === 'light';
}

export function createMarkdownTheme(colors: AppColors): MarkdownTheme {
  const dark = isDarkTheme(colors);

  return {
    // 整个 Markdown 区域的背景色，直接跟随当前主题表面色。
    background: colors.surface,
    // 卡片或表格等高亮容器的主表面色。
    surface: colors.surface,
    // 轻微区分层级时使用的浅表面色，基于品牌色叠加到当前表面色。
    surfaceSoft: mixColors(colors.surface, colors.brand, dark ? 0.14 : 0.06),
    // 常规边框颜色，直接复用当前主题边框。
    border: colors.border,
    // 更强调的边框颜色，在边框色上叠加正文色得到更高对比。
    borderStrong: mixColors(colors.border, colors.textPrimary, dark ? 0.32 : 0.14),
    // 正文主文字颜色，直接复用主题主文字。
    text: colors.textPrimary,
    // 次级文字颜色，直接复用主题次级文字。
    textSoft: colors.textSecondary,
    // 标题文字颜色，直接复用主题主文字。
    heading: colors.textPrimary,
    // 主强调色，直接复用品牌色。
    primary: colors.brand,
    // 主强调色的浅色背景版本，直接复用品牌浅色。
    primarySoft: colors.brandSoft,
    // 引用块背景色，在表面色上轻叠品牌色。
    quoteBg: mixColors(colors.surface, colors.brand, dark ? 0.18 : 0.08),
    // 代码块背景色，基于正文色和表面色混合，保证和整体主题一致。
    codeBg: mixColors(colors.surface, colors.textPrimary, dark ? 0.18 : 0.88),
    // 代码块文字颜色，亮主题下用表面色，暗主题下用主文字色。
    codeText: dark ? colors.textPrimary : colors.surface,
    // 表格隔行底色，在 muted surface 上轻叠品牌色。
    tableStripe: mixColors(colors.surfaceMuted, colors.brand, dark ? 0.12 : 0.04),
  };
}

const mono = Platform.select({
  // iOS 平台使用 Menlo 作为等宽字体。
  ios: 'Menlo',
  // Android 平台使用系统等宽字体。
  android: 'monospace',
  // 其他平台回退到通用等宽字体。
  default: 'monospace',
});

const serif = Platform.select({
  // iOS 平台使用 Georgia 作为衬线字体。
  ios: 'Georgia',
  // Android 平台使用系统衬线字体。
  android: 'serif',
  // 其他平台回退到通用衬线字体。
  default: 'serif',
});

export function createMarkdownStyles(colors: AppColors, tableCellMinWidth: number) {
  const theme = createMarkdownTheme(colors);
  return StyleSheet.create({
    // Markdown 根容器样式。
    body: {
      // 正文主文字颜色。
      color: theme.text,
      // Markdown 区域背景色。
      backgroundColor: theme.background,
      // 根容器默认使用衬线字体。
      fontFamily: serif,
      // 正文字号。
      fontSize: fontSizes.md,
      // 正文行高。
      lineHeight: 28,
    },
    // 默认文本节点样式。
    text: {
      // 正文主文字颜色。
      color: theme.text,
      // 文本默认使用衬线字体。
      fontFamily: serif,
      // 默认字号。
      fontSize: fontSizes.md,
      // 默认行高。
      lineHeight: 28,
    },
    // 段落块样式。
    paragraph: {
      // 段落顶部不额外留白。
      marginTop: 0,
    },
    // 一级标题样式。
    heading1: {
      // 标题颜色。
      color: theme.heading,
      // 使用衬线字体增强书卷感。
      fontFamily: serif,
      // 一级标题字号。
      fontSize: markdownHeadingFontSizes.h1,
      // 一级标题行高。
      lineHeight: 38,
      // 一级标题字重。
      fontWeight: '700',
      // 顶部留白。
      marginTop: 6,
      // 底部留白。
      marginBottom: 6,
      // 下方分隔线内边距。
      // 分隔线宽度。
      borderBottomWidth: StyleSheet.hairlineWidth,
      // 分隔线颜色。
      borderBottomColor: theme.borderStrong,
    },
    // 二级标题样式。
    heading2: {
      // 标题颜色。
      color: theme.heading,
      // 使用衬线字体。
      fontFamily: serif,
      // 二级标题字号。
      fontSize: markdownHeadingFontSizes.h2,
      // 二级标题行高。
      lineHeight: 32,
      // 二级标题字重。
      fontWeight: '700',
      // 顶部留白。
      marginTop: 10,
      // 底部留白。
      marginBottom: 4,
      // 分隔线宽度。
      borderBottomWidth: StyleSheet.hairlineWidth,
      // 分隔线颜色。
      borderBottomColor: theme.border,
    },
    // 三级标题样式。
    heading3: {
      // 标题颜色。
      color: theme.heading,
      // 使用衬线字体。
      fontFamily: serif,
      // 三级标题字号。
      fontSize: markdownHeadingFontSizes.h3,
      // 三级标题行高。
      lineHeight: 28,
      // 三级标题字重。
      fontWeight: '700',
      // 底部留白。
      marginBottom: 2,
    },
    // 四级标题样式。
    heading4: {
      // 标题颜色。
      color: theme.heading,
      // 使用衬线字体。
      fontFamily: serif,
      // 四级标题字号。
      fontSize: markdownHeadingFontSizes.h4,
      // 四级标题行高。
      lineHeight: 26,
      // 四级标题字重。
      fontWeight: '700',
      // 底部留白。
      marginBottom: 10,
    },
    // 五级标题样式。
    heading5: {
      // 标题颜色。
      color: theme.heading,
      // 使用衬线字体。
      fontFamily: serif,
      // 五级标题字号。
      fontSize: markdownHeadingFontSizes.h5,
      // 五级标题行高。
      lineHeight: 24,
      // 五级标题字重。
      fontWeight: '700',
      // 底部留白。
      marginBottom: 8,
    },
    // 六级标题样式。
    heading6: {
      // 使用更柔和的标题颜色。
      color: theme.textSoft,
      // 使用衬线字体。
      fontFamily: serif,
      // 六级标题字号。
      fontSize: markdownHeadingFontSizes.h6,
      // 六级标题行高。
      lineHeight: 22,
      // 六级标题字重。
      fontWeight: '700',
      // 顶部留白。
      marginTop: 16,
      // 底部留白。
      marginBottom: 8,
    },
    // 无序列表容器样式。
    bullet_list: {
      // 列表底部间距。
      marginBottom: 12,
    },
    // 有序列表容器样式。
    ordered_list: {
      // 列表底部间距。
      marginBottom: 12,
    },
    // 单个列表项样式。
    list_item: {
      // 每个列表项上下间距。
      marginVertical: 4,
    },
    // 无序列表圆点样式。
    bullet_list_icon: {
      // 圆点强调色。
      color: theme.primary,
    },
    // 有序列表序号样式。
    ordered_list_icon: {
      // 序号强调色。
      color: theme.primary,
      // 序号字重。
      fontWeight: '700',
    },
    // 行内代码样式。
    code_inline: {
      // 上下内边距。
      paddingVertical: 2,
      // 左右内边距。
      paddingHorizontal: 7,
      // 行内代码文字颜色。
      color: theme.primary,
      // 行内代码背景色。
      backgroundColor: theme.primarySoft,
      // 圆角大小。
      borderRadius: 7,
      // 等宽字体。
      fontFamily: mono,
      // 行内代码字号。
      fontSize: 14,
    },
    // 缩进代码块样式。
    code_block: {
      // 代码文字颜色。
      color: theme.codeText,
      // 代码背景色。
      backgroundColor: theme.codeBg,
      // 圆角大小。
      borderRadius: 14,
      // 左右内边距。
      paddingHorizontal: 18,
      // 上下内边距。
      paddingVertical: 16,
      // 等宽字体。
      fontFamily: mono,
      // 代码字号。
      fontSize: 14,
      // 代码行高。
      lineHeight: 24,
    },
    // 围栏代码块样式。
    fence: {
      // 代码文字颜色。
      color: theme.codeText,
      // 代码背景色。
      backgroundColor: theme.codeBg,
      // 圆角大小。
      borderRadius: 14,
      // 左右内边距。
      paddingHorizontal: 18,
      // 上下内边距。
      paddingVertical: 16,
      // 等宽字体。
      fontFamily: mono,
      // 代码字号。
      fontSize: 14,
      // 代码行高。
      lineHeight: 24,
    },
    // 水平分隔线样式。
    hr: {
      // 分隔线颜色。
      backgroundColor: theme.border,
      // 分隔线高度。
      height: StyleSheet.hairlineWidth,
      // 上下外边距。
      marginVertical: 2,
    },
    // 引用块样式。
    blockquote: {
      // 上下内边距。
      paddingVertical: 6,
      // 左右内边距。
      paddingHorizontal: 6,
      // 引用文字颜色。
      color: theme.textSoft,
      // 引用背景色。
      backgroundColor: theme.quoteBg,
      // 圆角大小。
      borderRadius: 14,
    },
    // 链接样式。
    link: {
      // 链接颜色。
      color: theme.primary,
      // 链接下划线。
      textDecorationLine: 'underline',
    },
    // 表格外框样式。
    table: {
      // 表格按内容宽度布局，实际宽度在渲染规则里统一计算。
      alignSelf: 'flex-start',
      // 表格背景色。
      backgroundColor: theme.surface,
      // 表格边框宽度。
      borderWidth: 1,
      // 表格边框颜色。
      borderColor: theme.border,
      // 表格圆角。
      borderRadius: 14,
      // 裁掉圆角外溢内容。
      overflow: 'hidden',
    },
    // 表头区域样式。
    thead: {
      // 表头背景色。
      backgroundColor: theme.surfaceSoft,
    },
    // 表格行样式。
    tr: {
      // 单元格横向排列。
      flexDirection: 'row',
      // 行底部分隔线宽度。
      borderBottomWidth: StyleSheet.hairlineWidth,
      // 行分隔线颜色。
      borderColor: theme.border,
    },
    // 表头单元格样式。
    th: {
      // 单元格最小宽度，保证列内容不会过度压缩。
      minWidth: tableCellMinWidth,
      // 默认宽度基线，便于在可用空间里自适应分配。
      flexBasis: tableCellMinWidth,
      // 列少时允许拉伸填满剩余空间。
      flexGrow: 1,
      // 不允许收缩宽度。
      flexShrink: 0,
      // 表头文字颜色。
      color: theme.heading,
      // 表头文字字重。
      fontWeight: '700',
      // 表头背景色。
      backgroundColor: theme.surfaceSoft,
      // 右侧分隔线宽度。
      borderRightWidth: StyleSheet.hairlineWidth,
      // 分隔线颜色。
      borderColor: theme.border,
    },
    // 表体单元格样式。
    td: {
      // 单元格最小宽度，保证列内容不会过度压缩。
      minWidth: tableCellMinWidth,
      // 默认宽度基线，便于在可用空间里自适应分配。
      flexBasis: tableCellMinWidth,
      // 列少时允许拉伸填满剩余空间。
      flexGrow: 1,
      // 不允许收缩宽度。
      flexShrink: 0,
      // 表体文字颜色。
      color: theme.textSoft,
      // 右侧分隔线宽度。
      borderRightWidth: StyleSheet.hairlineWidth,
      // 分隔线颜色。
      borderColor: theme.border,
    },
    // 加粗文本样式。
    strong: {
      // 强调文字颜色。
      color: theme.heading,
      // 强调文字字重。
      fontWeight: '700',
    },
    // 斜体文本样式。
    em: {
      // 斜体文字颜色。
      color: theme.text,
      // 斜体字形。
      fontStyle: 'italic',
    },
    // 删除线文本样式。
    s: {
      // 删除线文字颜色。
      color: theme.textSoft,
      // 删除线效果。
      textDecorationLine: 'line-through',
    },
  });
}

function countTableColumns(node: MarkdownNode) {
  let maxColumns = 0;

  for (const section of node.children ?? []) {
    for (const row of section.children ?? []) {
      const columnCount = (row.children ?? []).filter(
        (cell) => cell.type === 'th' || cell.type === 'td',
      ).length;
      maxColumns = Math.max(maxColumns, columnCount);
    }
  }

  return Math.max(maxColumns, 1);
}

export function createMarkdownRules(tableCellMinWidth: number, tableViewportWidth: number): RenderRules {
  return {
    // 在窄屏上为宽表格提供横向滚动容器。
    table: (node, children, _parent, styles) => {
      const columnCount = countTableColumns(node as MarkdownNode);
      const tableWidth = Math.max(tableViewportWidth, columnCount * tableCellMinWidth);

      return (
        <ScrollView
          // 使用 AST 节点 key 保持渲染稳定。
          key={node.key}
          // 开启横向滚动。
          horizontal
          // 允许在嵌套滚动场景下工作。
          nestedScrollEnabled
          // 关闭 iOS 回弹效果。
          bounces={false}
          // 显示横向滚动条，提示用户可左右拖动。
          showsHorizontalScrollIndicator>
          <View
            // 统一整张表格宽度，避免每一行按各自内容单独拉伸导致列错位。
            style={[styles._VIEW_SAFE_table, {width: tableWidth}]}>
            {children}
          </View>
        </ScrollView>
      );
    },
  };
}
