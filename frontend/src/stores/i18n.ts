import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

export type Locale = 'zh' | 'en'

const LOCALE_KEY = 'sb-fox-locale'
const DEFAULT_LOCALE: Locale = 'zh'

const en: Record<string, string> = {
  '仪表盘': 'Dashboard',
  '节点': 'Nodes',
  '模板': 'Templates',
  '订阅分组': 'Profiles',
  '订阅': 'Subscriptions',
  '预览生成': 'Preview',
  '用户': 'Users',
  '设置': 'Settings',
  '语言': 'Language',
  '新建用户': 'New User',
  '编辑用户': 'Edit User',
  '用户名': 'Username',
  '角色': 'Role',
  '节点上限': 'Node Limit',
  '分组上限': 'Profile Limit',
  '模板上限': 'Template Limit',
  '操作': 'Actions',
  '不限': 'Unlimited',
  '关闭': 'Close',
  '复制': 'Copy',
  '复制成功': 'Copied',
  '复制失败': 'Copy failed',
  '的新密码': "'s new password",
  '初始密码': 'Initial Password',
  '取消': 'Cancel',
  '保存': 'Save',
  '新建节点': 'New Node',
  '编辑节点': 'Edit Node',
  '协议类型': 'Protocol',
  '标签': 'Tag',
  '服务器': 'Server',
  '端口': 'Port',
  '协议参数': 'Protocol Options',
  '加密方式': 'Method',
  '密码': 'Password',
  'obfs 类型': 'OBFS Type',
  '启用 TLS': 'Enable TLS',
  '服务名': 'Server Name',
  '逗号分隔': 'Comma separated',
  '允许不安全': 'Allow insecure',
  'uTLS 指纹': 'uTLS Fingerprint',
  '国家': 'Country',
  '手动指定国家': 'Set Country Manually',
  '未指定': 'Unspecified',
  '输入': 'Input',
  '选择模板': 'Select Template',
  '自动国家分组': 'Auto Country Groups',
  '链式代理': 'Chain Proxy',
  '生成': 'Generate',
  '校验': 'Validate',
  '格式化': 'Format',
  '配置输出': 'Config Output',
  '点击「生成」查看配置。': 'Click "Generate" to preview the config.',
  '可用': 'Available',
  '不可用': 'Unavailable',
  '内核状态': 'Kernel Status',
  '未配置': 'Not configured',
  '节点国家分布': 'Node Country Distribution',
  '暂无节点': 'No nodes',
  '刷新国家': 'Refresh Country',
  '导出模板': 'Export Template',
  '导入': 'Import',
  '新建': 'New',
  '全部来源': 'All Sources',
  '协议导入': 'Protocol',
  '订阅导入': 'Subscription',
  '配置导入': 'Config',
  '手动创建': 'Manual',
  '全部国家': 'All Countries',
  '全部类型': 'All Types',
  '全选': 'Select All',
  '全不选': 'Deselect All',
  '取消全选': 'Clear Selection',
  '已选': 'Selected',
  '清空': 'Clear',
  '搜索节点...': 'Search nodes...',
  '搜索 tag / server...': 'Search tag / server...',
  '无匹配节点': 'No matching nodes',
  '有效': 'Valid',
  '无效': 'Invalid',
  '内核不可用': 'Kernel unavailable',
  '请先安装 sing-box 内核或在设置中配置路径': 'Install sing-box or configure the kernel path in Settings',
  '暂无节点，点击「导入」或「新建」添加。': 'No nodes. Click "Import" or "New" to add one.',
  '导入节点': 'Import Nodes',
  '分享链接': 'Share Links',
  '订阅 URL': 'Subscription URL',
  '每行一个 ss/vmess/vless/trojan/hysteria2/tuic 链接，或粘贴 base64 订阅内容。':
    'One ss/vmess/vless/trojan/hysteria2/tuic link per line, or paste base64 subscription content.',
  '名称': 'Name',
  '我的订阅': 'My Subscription',
  '服务端抓取，默认拒绝私网地址。':
    'Fetched by the server. Private network addresses are denied by default.',
  '粘贴完整 config.json，将从 outbounds 中提取代理节点，跳过 selector/direct 等分组。':
    'Paste a complete config.json. Proxy nodes are extracted from outbounds; selector/direct groups are skipped.',
  '新建分组': 'New Profile',
  '编辑分组': 'Edit Profile',
  '暂无分组。': 'No profiles.',
  '新建订阅': 'New Subscription',
  '编辑订阅': 'Edit Subscription',
  '暂无订阅。': 'No subscriptions.',
  '公开订阅链接': 'Public Subscription URL',
  '轮换 token': 'Rotate token',
  '模板:': 'Template:',
  '个节点': 'nodes',
  '个组合': 'groups',
  '组合节点': 'Node Groups',
  '新建组合': 'New Group',
  '新建组合节点': 'New Node Group',
  '编辑组合节点': 'Edit Node Group',
  '节点列表': 'Node List',
  '暂无组合节点。': 'No node groups.',
  '单节点': 'Single Nodes',
  '链式代理节点': 'Chain Proxy Node',
  '链式代理需要至少一个上游节点': 'Chain proxy needs at least one upstream node',
  '选择节点': 'Select Node',
  '导入模板': 'Import Template',
  '编辑模板': 'Edit Template',
  '类型': 'Type',
  '描述': 'Description',
  '查看': 'View',
  '出口分组': 'Group Management',
  '分组管理': 'Group Management',
  '导出': 'Export',
  '添加分组': 'Add Group',
  'final 使用的 selector': 'Final Selector',
  '出口': 'Outbounds',
  '无可选出口': 'No available outbounds',
  '默认出口': 'Default Outbound',
  '拖拽排序': 'Drag to reorder',
  '添加子 selector': 'Add Child Selector',
  '仍被引用，不能删除': 'Still referenced, cannot delete',
  '引用:': 'References:',
  '删除': 'Delete',
  '未检测到 selector/urltest 分组。': 'No selector/urltest groups detected.',
  '上传 JSON 文件': 'Upload JSON file',
  '模板内容': 'Template Content',
  '无出口': 'No outbounds',
  '显示名称': 'Display Name',
  'sing-box 内核': 'sing-box Kernel',
  '状态:': 'Status:',
  '内核路径': 'Kernel Path',
  '测试': 'Test',
  '国家热度排序': 'Country Heat Order',
  '重置': 'Reset',
  '上移': 'Move Up',
  '下移': 'Move Down',
  '个国家/地区': 'countries/regions',
  '订阅抓取': 'Subscription Fetching',
  '允许抓取私网地址': 'Allow fetching private network addresses',
  '关闭时会拒绝私网、环回、链路本地、CGNAT、组播和云元数据地址；仅在可信内网订阅源中开启。':
    'When disabled, private, loopback, link-local, CGNAT, multicast, and cloud metadata addresses are refused. Enable only for trusted private subscription sources.',
  '默认关闭以防止 SSRF。仅在受信任的内网环境中开启。':
    'Disabled by default to prevent SSRF. Enable only in trusted private networks.',
  '修改密码': 'Change Password',
  '当前密码': 'Current Password',
  '新密码至少 8 位': 'New Password, at least 8 chars',
  '确认新密码': 'Confirm New Password',
  '登录': 'Login',
  '注册': 'Register',
  'sing-box 配置订阅管理': 'sing-box config subscription manager',
}

function readLocale(): Locale {
  try {
    return localStorage.getItem(LOCALE_KEY) === 'en' ? 'en' : DEFAULT_LOCALE
  } catch (e) {
    console.warn('sb-fox: unable to read locale preference', e)
    return DEFAULT_LOCALE
  }
}

function saveLocale(locale: Locale): void {
  try {
    localStorage.setItem(LOCALE_KEY, locale)
  } catch (e) {
    console.warn('sb-fox: unable to save locale preference', e)
  }
}

export const useI18nStore = defineStore('i18n', () => {
  const locale = ref<Locale>(readLocale())
  const isEnglish = computed(() => locale.value === 'en')

  function t(key: string): string {
    return locale.value === 'en' ? en[key] || key : key
  }

  function toggleLocale(): void {
    locale.value = locale.value === 'en' ? 'zh' : 'en'
    saveLocale(locale.value)
  }

  return { locale, isEnglish, t, toggleLocale }
})
