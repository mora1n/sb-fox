import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

export type Locale = 'zh' | 'en'

const LOCALE_KEY = 'sb-fox-locale'
const DEFAULT_LOCALE: Locale = 'zh'

const en: Record<string, string> = {
  '仪表盘': 'Dashboard',
  '节点': 'Nodes',
  '模板': 'Templates',
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
  '复制节点': 'Duplicate Node',
  '复制节点需要修改标签后保存': 'Change the node tag before saving the duplicate',
  '协议类型': 'Protocol',
  '标签': 'Tag',
  '服务器': 'Server',
  '端口': 'Port',
  '协议参数': 'Protocol Options',
  '加密方式': 'Method',
  '密码': 'Password',
  '显示密码': 'Show Password',
  '隐藏密码': 'Hide Password',
  'obfs 类型': 'OBFS Type',
  '启用 TLS': 'Enable TLS',
  '服务名': 'Server Name',
  '逗号分隔': 'Comma separated',
  '允许不安全': 'Allow insecure',
  'uTLS 指纹': 'uTLS Fingerprint',
  '国家': 'Country',
  '来源': 'Source',
  '手动指定国家': 'Set Country Manually',
  '未指定': 'Unspecified',
  '输入': 'Input',
  '选择模板': 'Select Template',
  '自动国家分组': 'Auto Country Groups',
  '自动国家分组来源': 'Auto Country Source',
  '生成选项': 'Generation Options',
  '开启': 'On',
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
  '模板 JSON': 'Template JSON',
  '协议链接': 'Protocol Links',
  '导入': 'Import',
  '新建': 'New',
  '全部来源': 'All Sources',
  '协议导入': 'Protocol',
  '订阅导入': 'Subscription',
  '配置导入': 'Config',
  '手动创建': 'Manual',
  '全部国家': 'All Countries',
  '全部类型': 'All Types',
  '全部协议': 'All Protocols',
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
  '请选择有效 sing-box 内核或联系管理员配置内核':
    'Select a valid sing-box kernel or ask an admin to configure one',
  '暂无节点，点击「导入」或「新建」添加。': 'No nodes. Click "Import" or "New" to add one.',
  '导入节点': 'Import Nodes',
  '分享链接': 'Share Links',
  '订阅 URL': 'Subscription URL',
  '每行一个 ss/vmess/vless/trojan/hysteria2/tuic/naive 链接，或粘贴 base64、SIP008、Clash/Mihomo 订阅内容。':
    'One ss/vmess/vless/trojan/hysteria2/tuic/naive link per line, or paste base64, SIP008, Clash/Mihomo subscription content.',
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
  '复制订阅': 'Duplicate Subscription',
  '复制订阅需要修改名称后保存': 'Change the subscription name before saving the duplicate',
  '暂无订阅。': 'No subscriptions.',
  '公开订阅链接': 'Public Subscription URL',
  '轮换 token': 'Rotate token',
  '共享 token': 'Shared token',
  '轮换共享 token': 'Rotate shared token',
  '所有订阅链接共享同一 token，并按订阅名称区分。':
    'All subscription URLs share one token and are distinguished by subscription name.',
  '订阅链接': 'Subscription URL',
  '订阅 Host': 'Subscription Host',
  '订阅链接前缀': 'Subscription URL prefix',
  '留空时使用当前浏览器 Host。': 'Leave empty to use the current browser host.',
  '出口选择': 'Outlet Selection',
  '链式代理节点': 'Chain Proxy Nodes',
  '当前出口分组': 'Current Outlet Group',
  '引用出口': 'Referenced Outbounds',
  '不能选择会造成循环引用的分组': 'Cannot select a group that would create a reference cycle',
  '跳过国家分组': 'Skip Country Groups',
  '模板:': 'Template:',
  '个节点': 'nodes',
  '个组合': 'groups',
  '组合节点': 'Node Groups',
  '卡片': 'Cards',
  '列表': 'List',
  '新建组合': 'New Group',
  '新建组合节点': 'New Node Group',
  '编辑组合节点': 'Edit Node Group',
  '复制组合节点': 'Duplicate Node Group',
  '复制组合节点需要修改名称后保存': 'Change the node group name before saving the duplicate',
  '节点列表': 'Node List',
  '暂无组合节点。': 'No node groups.',
  '单节点': 'Single Nodes',
  '链式代理需要至少一个上游节点': 'Chain proxy needs at least one upstream node',
  '选择节点': 'Select Node',
  '导入模板': 'Import Template',
  '编辑模板': 'Edit Template',
  '复制模板': 'Duplicate Template',
  '复制模板需要修改名称后保存': 'Change the template name before saving the duplicate',
  '模板名称已存在': 'Template name already exists',
  '暂无模板。': 'No templates.',
  '类型': 'Type',
  '描述': 'Description',
  '查看': 'View',
  '出口分组': 'Group Management',
  '分组管理': 'Group Management',
  '导出': 'Export',
  '添加分组': 'Add Group',
  'final 使用的 selector': 'Final Selector',
  '最终出口': 'Final Outbound',
  '使用 sing-box 默认': 'Use sing-box default',
  '出口': 'Outbounds',
  '无可选出口': 'No available outbounds',
  '默认出口': 'Default Outbound',
  '拖拽排序': 'Drag to reorder',
  '添加子 selector': 'Add Child Selector',
  '仍被引用，不能删除': 'Still referenced, cannot delete',
  '引用:': 'References:',
  '删除': 'Delete',
  '选项': 'Options',
  '未检测到 selector/urltest 分组。': 'No selector/urltest groups detected.',
  '上传 JSON 文件': 'Upload JSON file',
  '模板内容': 'Template Content',
  '无出口': 'No outbounds',
  '显示名称': 'Display Name',
  'sing-box 内核': 'sing-box Kernel',
  '状态:': 'Status:',
  '内核路径': 'Kernel Path',
  '内核名称': 'Kernel Name',
  '添加内核': 'Add Kernel',
  '删除内核': 'Remove Kernel',
  '当前内核': 'Current Kernel',
  '选择内核': 'Select Kernel',
  '未测试': 'Untested',
  '内核已切换': 'Kernel switched',
  '内核已保存': 'Kernels saved',
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
  '新密码至少 4 位': 'New password, at least 4 chars',
  '确认新密码': 'Confirm New Password',
  '登录': 'Login',
  '注册': 'Register',
  'sing-box 配置订阅管理': 'sing-box config subscription manager',
  '正在加载订阅...': 'Loading subscription...',
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
