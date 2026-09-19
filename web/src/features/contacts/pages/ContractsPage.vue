<template>
  <div class="cp-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">合同管理</h2>
        <p class="page-sub">共 {{ contracts.length }} 份合同</p>
      </div>
      <div class="cp-header-actions">
        <van-button size="small" icon="downloader" @click="exportZip">导出</van-button>
        <van-button type="primary" size="small" icon="plus" @click="openUpload(null)">上传合同</van-button>
      </div>
    </div>

    <!-- 合同预警统计卡 -->
    <div class="cp-stats">
      <div v-for="card in EXPIRY_CARDS" :key="card.key" class="cp-stat-card"
        :class="{ 'cp-stat-active': expiryFilter === card.key }"
        @click="expiryFilter = card.key">
        <div class="cp-stat-label">{{ card.label }}</div>
        <div class="cp-stat-num">{{ card.key === 'all' ? expiryStats.total : expiryStats[card.key] }}</div>
      </div>
    </div>

    <div class="cp-filters">
      <van-search v-model="keyword" placeholder="搜索文件名 / 单位名" shape="round" background="transparent" class="cp-search" />
      <select v-model="activePartyType" class="cp-select">
        <option value="all">全部单位类型</option>
        <option value="invest">长期投资单位</option>
        <option value="reinvest">再投资单位</option>
        <option value="flow">土地流转企业</option>
        <option value="other">其他单位</option>
      </select>
    </div>

    <van-loading v-if="loading && grouped.length === 0" />
    <div v-else-if="grouped.length === 0" class="empty">
      <van-empty description="暂无合同" />
    </div>

    <div v-for="g in grouped" :key="g.partyId" class="cp-group">
      <div class="cp-group-header" @click="toggleExpand(g.partyId)">
        <div class="cp-group-left">
          <span class="cp-chev">{{ isExpanded(g.partyId) ? '▼' : '▶' }}</span>
          <span class="cp-party-name">{{ g.partyName }}</span>
          <span class="cp-party-type" :class="'tag-' + g.partyType">{{ partyTypeLabel(g.partyType) }}</span>
          <span class="cp-count">{{ g.files.length }}</span>
          <span class="party-expiry-tag" :class="partyStatusClass(g.partyId)">{{ partyStatusText(g.partyId) }}</span>
        </div>
        <van-button size="mini" plain icon="plus" @click.stop="openUpload(g.partyId)">上传到此单位</van-button>
      </div>
      <div v-show="isExpanded(g.partyId)" class="cp-group-body">
        <div v-if="g.files.length === 0" class="cp-empty-group">（暂无合同）</div>
        <div v-for="c in g.files" :key="c.id" class="cp-file-item">
          <span class="cp-file-ic" :class="isArchivedContract(c) ? 'ic-archived' : 'ic-' + fileExt(c.fileName)">
            {{ isArchivedContract(c) ? '📦' : '📄' }}
          </span>
          <div class="cp-file-info">
            <div class="cp-file-name" :class="{ 'cp-file-archived': isArchivedContract(c) }" @click="viewContract(c)">
              {{ isArchivedContract(c) ? '到期续签新合同，已归档' : c.fileName }}
            </div>
            <div class="cp-file-meta">
              {{ fileExt(c.fileName).toUpperCase() }} · {{ fmtSize(c.fileSize) }} · {{ c.uploadedAt }}
            </div>
            <div class="cp-file-expiry" :class="contractExpiryClass(c)">
              <span class="cfe-label">合同到期</span>
              <span class="cfe-date">{{ c.expiresAt || '—' }}</span>
              <span v-if="c.expiresAt" class="cfe-state">{{ contractExpiryText(c) }}</span>
            </div>
          </div>
          <div class="cp-file-actions">
            <van-button size="mini" @click="viewContract(c)">查看</van-button>
            <van-button size="mini" plain @click.stop="openEditExpiry(c)">改期</van-button>
            <van-button size="mini" type="danger" plain @click="remove(c)">删除</van-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 上传 dialog -->
    <van-dialog v-model:show="dialogVisible" title="上传合同" show-cancel-button :confirm-button-text="'确认上传'"
      @confirm="submitUpload" @cancel="dialogVisible = false">
      <div class="up-body">
        <div v-if="uploadPartyId !== null && uploadPartyId !== undefined" class="up-field">
          <label>所属单位</label>
          <div class="up-party-fixed">{{ partyName(uploadPartyId) }}</div>
        </div>
        <div v-else class="up-field">
          <label>所属单位</label>
          <select v-model.number="uploadPartyChosen" class="up-select">
            <option :value="null" disabled>请选择所属单位</option>
            <option v-for="p in parties" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div class="up-field">
          <label>选择文件</label>
          <div class="up-drop" @click="triggerFile" @dragover.prevent @drop.prevent="onDrop">
            <template v-if="!chosenFile">
              <div class="up-drop-ic">📁</div>
              <div>点击选择 或 拖拽文件到此处</div>
              <div class="up-drop-sub">合同只能上传 PDF 格式，单个 ≤ 10MB</div>
            </template>
            <template v-else>
              <div class="up-chosen">
                <span>📎 {{ chosenFile.name }}</span>
                <van-button size="mini" plain type="danger" @click.stop="chosenFile = null">移除</van-button>
              </div>
            </template>
          </div>
          <input ref="fileInputRef" type="file" accept=".pdf,application/pdf" @change="onPickFile" style="display:none" />
        </div>
        <div class="up-field">
          <label>合同名称</label>
          <input class="up-name-input" v-model="manualName" placeholder="填合同名称，留空自动按类型命名（如：图片合同）" />
        </div>
        <div class="up-field">
          <label>合同期至（可选）</label>
          <input type="date" v-model="contractExpiresAt" class="up-name-input" />
          <div class="up-hint">选择后上传会登记到期提醒；到期前 30 天及到期后将在「往来单位」角标提示</div>
        </div>
      </div>
    </van-dialog>

    <!-- 合同期至调整弹窗 -->
    <van-dialog v-model:show="editExpiryVisible" title="合同期至调整" show-cancel-button :confirm-button-text="'保存'"
      @confirm="saveContractExpiry" @cancel="editExpiryVisible = false">
      <div class="ee-body">
        <div class="ee-sub">{{ editingContract ? editingContract.fileName : '' }}</div>
        <input type="date" v-model="editExpiryDate" class="ee-date" />
        <div class="ee-tip">留空并保存 = 清除到期提醒</div>
      </div>
    </van-dialog>

    <!-- 删除合同二次验证（倒计时 5 秒后才可确认删除） -->
    <van-dialog v-model:show="delVisible" title="删除合同" :show-confirm-button="false"
      @closed="closeDeleteDialog">
      <div class="dc-body">
        <div class="dc-file">{{ pendingDelete?.fileName }}</div>
        <div class="dc-warn">该操作不可恢复，删除后无法找回原合同，请谨慎！</div>
        <div class="dc-count" v-if="delCountdown > 0">{{ delCountdown }} 秒后可确认删除</div>
        <div class="dc-btns">
          <van-button size="small" @click="closeDeleteDialog">取消</van-button>
          <van-button size="small" :disabled="delCountdown > 0" @click="confirmDelete">确定删除</van-button>
        </div>
      </div>
    </van-dialog>

    <!-- 预览弹窗（放弃 van-dialog 手写 div，彻底绕开 Vant 的 z-index/transform/遮罩 坑） -->
    <Teleport to="body">
      <div v-if="previewVisible" class="preview-dialog" tabindex="-1">
        <div class="preview-header">
          <span class="preview-title">{{ previewFileName }}</span>
          <button class="preview-close-btn" @click="previewVisible = false">关闭</button>
        </div>
        <div v-if="previewLoading" class="preview-body">
          <van-loading size="36px" color="var(--info)" />
          <p>正在加载 {{ previewFileName }}...</p>
        </div>
        <div v-else-if="previewFileData" class="preview-body">
          <!-- 文本 -->
          <pre v-if="previewKind === 'text'" class="preview-text">{{ previewTextContent }}</pre>
          <!-- 图片 -->
          <img v-else-if="previewKind === 'image'" :src="previewFileData" class="preview-img" />
          <!-- Word (.docx) — mammoth 转 HTML -->
          <div v-else-if="previewKind === 'word'" class="preview-docx" v-html="previewHtmlContent"></div>
          <!-- Excel (.xlsx) — xlsx 转表格 -->
          <div v-else-if="previewKind === 'excel'" class="preview-excel">
            <div class="excel-sheet-tabs" v-if="excelSheets.length > 1">
              <button v-for="(s, i) in excelSheets" :key="i" class="excel-tab" :class="{ active: excelActiveSheet === i }" @click="excelActiveSheet = i">{{ s }}</button>
            </div>
            <div class="excel-scroll">
              <table class="excel-table" v-html="excelHtml"></table>
            </div>
          </div>
          <!-- 老式 Office / PPT / .xls 等无法在浏览器原生渲染的 -->
          <div v-else-if="previewKind === 'office-download'" class="preview-office-tip">
            <div class="office-ic">📄</div>
            <p>浏览器无法直接预览 <b>{{ previewFileName }}</b></p>
            <p class="office-sub">请下载后用 Microsoft Office 或 WPS 打开</p>
            <a :href="previewFileData" :download="previewFileName" class="download-btn">⬇ 立即下载 {{ previewFileName }}</a>
          </div>
          <!-- 其他：iframe（PDF 等浏览器原生支持的 MIME） -->
          <iframe v-else :src="previewFileData" class="preview-iframe"></iframe>
        </div>
        <div v-else class="preview-body preview-empty">暂无文件内容</div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import mammoth from 'mammoth'
import * as XLSX from 'xlsx'
import { api } from '../../../lib/http'
import JSZip from 'jszip'
import { useContactIssues } from '../useContactIssues'

// 统一从 API 响应里提取数组
function extractList(r: any) {
  if (r == null) return [];
  if (Array.isArray(r)) return r;
  if (Array.isArray(r.data)) return r.data;
  if (r.data && Array.isArray(r.data.items)) return r.data.items;
  return [];
}

interface Party { id: number; name: string; type: string }
interface Contract { id: number; partyId: number; fileName: string; fileSize: number; mimeType?: string; uploadedAt: string; fileData?: string; expiresAt?: string | null }

const parties = ref<Party[]>([])
const contracts = ref<Contract[]>([])
const loading = ref(false)
const keyword = ref('')
const activePartyType = ref('all')
const expiryFilter = ref<'all' | 'expired' | 'pending' | 'future'>('all')

const dialogVisible = ref(false)
const uploadPartyId = ref<number | null>(null)
const uploadPartyChosen = ref<number | null>(null)
const chosenFile = ref<File | null>(null)
const manualName = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)

// 合同期至（可选）：上传时登记到期日
const contractExpiresAt = ref('')

// 合同期至调整弹窗
const editExpiryVisible = ref(false)

// 删除二次验证：倒计时 5 秒后才可确认删除
const delVisible = ref(false)
const delCountdown = ref(0)
const pendingDelete = ref<Contract | null>(null)
let delTimer: number | null = null
const editingContract = ref<Contract | null>(null)
const editExpiryDate = ref('')
const { refresh: refreshContactIssues } = useContactIssues()

// 预览相关
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewFileData = ref<string>('')
const previewFileName = ref<string>('')
const previewMime = ref<string>('')
const previewKind = ref<'text' | 'image' | 'word' | 'excel' | 'office-download' | 'iframe'>('iframe')
const previewTextContent = ref<string>('')
const previewHtmlContent = ref<string>('')

// Excel 多 sheet 支持
const excelSheets = ref<string[]>([])
const excelActiveSheet = ref(0)
const excelWorkbook = ref<any>(null)

// 无法在浏览器原生渲染的 Office 类型
const OFFICE_DOWNLOAD_MIMES = [
  'application/msword',                           // .doc (老式二进制)
  'application/vnd.ms-excel',                    // .xls (老式二进制)
  'application/vnd.ms-powerpoint',               // .ppt
  'application/vnd.openxmlformats-officedocument.presentationml.presentation', // .pptx
]
const OFFICE_DOWNLOAD_EXT = ['doc', 'xls', 'ppt', 'pptx']

function fmtSize(n: number) {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1024 / 1024).toFixed(1) + ' MB'
}
const fileExt = (name: string) => (name.split('.').pop() || '').toLowerCase()
const partyTypeLabel = (t?: string) => ({ invest: '长投', reinvest: '再投', flow: '流转', other: '其他' }[t || 'other'] || '其他')
const partyName = (id: number) => parties.value.find(p => p.id === id)?.name || `单位#${id}`

interface Group { partyId: number; partyName: string; partyType: string; files: Contract[] }

// 展开状态独立于 grouped 派生结构，用单位 id 列表控制（默认全折叠）
const expandedIds = ref<number[]>([])
function isExpanded(id: number) { return expandedIds.value.includes(id) }
function toggleExpand(id: number) {
  expandedIds.value = expandedIds.value.includes(id)
    ? expandedIds.value.filter(x => x !== id)
    : [...expandedIds.value, id]
}

// 合同列表按「全部单位」分组展示：每个单位都显示一组（含暂无合同的单位），
// 合同按所属单位归入各自组。
const grouped = computed<Group[]>(() => {
  // 1) 单位维度：类型过滤
  let plist = parties.value
  if (activePartyType.value !== 'all') {
    plist = plist.filter(p => p.type === activePartyType.value)
  }
  // 2) 生成单位骨架组
  const byParty = new Map<number, Group>()
  for (const p of plist) {
    byParty.set(p.id, { partyId: p.id, partyName: p.name, partyType: p.type, files: [] })
  }
  // 3) 把合同归入对应单位
  for (const c of contracts.value) {
    const g = byParty.get(c.partyId)
    if (g) g.files.push(c)
  }
  // 4) 关键词过滤（匹配单位名 或 单位下任一合同文件名）
  let arr = Array.from(byParty.values())
  const k = keyword.value.trim().toLowerCase()
  if (k) {
    arr = arr.filter(g =>
      g.partyName.toLowerCase().includes(k) ||
      g.files.some(f => f.fileName.toLowerCase().includes(k))
    )
  }
  // 5) 到期分类过滤（点击统计卡）：仅保留命中该分类的单位
  if (expiryFilter.value !== 'all') {
    arr = arr.filter(g => partyInCat(g.partyId, expiryFilter.value as ExpiryCat))
  }
  return arr.sort((a, b) => a.partyName.localeCompare(b.partyName, 'zh'))
})

// ============ base64 工具 ============
/** 把 data:xxx;base64,AAAA... 里的 base64 部分转成 Uint8Array */
function base64DataUrlToUint8(dataUrl: string): Uint8Array {
  const commaIdx = dataUrl.indexOf(',')
  const base64 = commaIdx >= 0 ? dataUrl.substring(commaIdx + 1) : dataUrl
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

// ============ 类型判定 ============
function detectPreviewKind(mime: string, fileName: string): 'text' | 'image' | 'word' | 'excel' | 'office-download' | 'iframe' {
  const ext = fileExt(fileName)
  if (mime.startsWith('text/')) return 'text'
  if (mime.startsWith('image/')) return 'image'
  if (mime === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' || ext === 'docx') return 'word'
  if (mime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' || ext === 'xlsx') return 'excel'
  if (OFFICE_DOWNLOAD_MIMES.includes(mime) || OFFICE_DOWNLOAD_EXT.includes(ext)) return 'office-download'
  return 'iframe' // PDF 等浏览器原生支持
}

// ============ 异步渲染器 ============
async function renderWord(dataUrl: string) {
  const bytes = base64DataUrlToUint8(dataUrl)
  const result = await mammoth.convertToHtml({ arrayBuffer: bytes.buffer })
  previewHtmlContent.value = result.value
  // mammoth 可以继续解析 warnings，但我们只需要 HTML
}

async function renderExcel(dataUrl: string) {
  const bytes = base64DataUrlToUint8(dataUrl)
  const wb = XLSX.read(bytes, { type: 'array', cellDates: true })
  excelWorkbook.value = wb
  excelSheets.value = wb.SheetNames
  excelActiveSheet.value = 0
}

// 当前激活的 sheet 渲染成 HTML table
const excelHtml = computed(() => {
  const wb = excelWorkbook.value
  if (!wb) return ''
  const sheetName = wb.SheetNames[excelActiveSheet.value]
  const ws = wb.Sheets[sheetName]
  // 简单方式：用 XLSX.write 输出 HTML
  return XLSX.utils.sheet_to_html(ws, { editable: false })
})

// ============ 主逻辑 ============
async function load() {
  loading.value = true
  try {
    const [p, c] = await Promise.all([
      api.get<{ data: Party[] } | Party[]>('/parties'),
      api.get<{ data: Contract[] } | Contract[]>('/contracts'),
    ])
    parties.value = extractList(p)
    contracts.value = extractList(c)
  } finally { loading.value = false }
}

// 东八区当日 YYYY-MM-DD（避开本地时区）
function cnToday(): string {
  return new Date(Date.now() + 8 * 3600 * 1000).toISOString().slice(0, 10)
}

// ============ 到期分类 ============
type ExpiryCat = 'expired' | 'pending' | 'future'
// 单合同分类：已到期(<0) / 即将到期(0~30) / 未到期(>30)；无到期日 → null
function contractExpiryCat(c: Contract): ExpiryCat | null {
  if (!c.expiresAt) return null
  const [y, m, d] = c.expiresAt.split('-').map(Number)
  const [ty, tm, td] = cnToday().split('-').map(Number)
  const days = Math.round((Date.UTC(y, m - 1, d) - Date.UTC(ty, tm - 1, td)) / 86400000)
  if (days < 0) return 'expired'
  if (days <= 30) return 'pending'
  return 'future'
}
// 单位是否命中某到期分类：以该单位所有合同的「最新期至」归类（只看最新期至，旧过期合同忽略）
function partyInCat(partyId: number, cat: ExpiryCat): boolean {
  return partyLatestCat(partyId) === cat
}

// 单位最新期至的分类：取该单位所有有期至合同的最大 expiresAt（字符串 YYYY-MM-DD 可直接比较），
// 按此归类；单位无任何期至合同 → null
function partyLatestCat(partyId: number): ExpiryCat | null {
  let latest: string | null = null
  for (const c of contracts.value) {
    if (c.partyId !== partyId || !c.expiresAt) continue
    if (latest === null || c.expiresAt > latest) latest = c.expiresAt
  }
  if (latest === null) return null
  const [y, m, d] = latest.split('-').map(Number)
  const [ty, tm, td] = cnToday().split('-').map(Number)
  const days = Math.round((Date.UTC(y, m - 1, d) - Date.UTC(ty, tm - 1, td)) / 86400000)
  if (days < 0) return 'expired'
  if (days <= 30) return 'pending'
  return 'future'
}

// 单位行到期状态文案：无期至→无到期日；否则按最新期至(30天口径)
function partyStatusText(partyId: number): string {
  const days = partyLatestDays(partyId)
  if (days === null) return '无到期日'
  if (days < 0) return `已到期${-days}天`
  if (days === 0) return '今天到期'
  return `还剩${days}天到期`
}

// 单位行到期状态着色：到期→红；临期(30天内)→橙；未到期→默认
function partyStatusClass(partyId: number): string {
  const days = partyLatestDays(partyId)
  if (days === null) return ''
  if (days < 0) return 'pet-expired'
  if (days <= 30) return 'pet-pending'
  return 'pet-ok'
}

// 单位最新期至剩余天数；无期至 → null
function partyLatestDays(partyId: number): number | null {
  const cat = partyLatestCat(partyId)
  if (cat === null) return null
  let latest: string | null = null
  for (const c of contracts.value) {
    if (c.partyId !== partyId || !c.expiresAt) continue
    if (latest === null || c.expiresAt > latest) latest = c.expiresAt
  }
  if (latest === null) return null
  const [y, m, d] = latest.split('-').map(Number)
  const [ty, tm, td] = cnToday().split('-').map(Number)
  return Math.round((Date.UTC(y, m - 1, d) - Date.UTC(ty, tm - 1, td)) / 86400000)
}

// ============ 单个合同到期展示（合同维度） ============
// 单个合同剩余天数；无期至 → null
function contractExpiryDays(c: Contract): number | null {
  if (!c.expiresAt) return null
  const [y, m, d] = c.expiresAt.split('-').map(Number)
  const [ty, tm, td] = cnToday().split('-').map(Number)
  return Math.round((Date.UTC(y, m - 1, d) - Date.UTC(ty, tm - 1, td)) / 86400000)
}

// 单个合同到期文案：已到期N天 / 今天到期 / 还剩N天到期；无期至 → ''
function contractExpiryText(c: Contract): string {
  const days = contractExpiryDays(c)
  if (days === null) return ''
  if (days < 0) return `已到期${-days}天`
  if (days === 0) return '今天到期'
  return `还剩${days}天到期`
}

// 单个合同到期着色：到期前 30 天（含已到期）标红；>30 天未到期默认；无期至置灰
function contractExpiryClass(c: Contract): string {
  const days = contractExpiryDays(c)
  if (days === null) return 'ce-none'
  if (days <= 30) return 'ce-warn'
  return 'ce-ok'
}

// 是否「到期续签后归档」：该合同已到期（<0天），且同单位存在期至更晚的续签合同
function isArchivedContract(c: Contract): boolean {
  if (!c.expiresAt) return false
  const [y, m, d] = c.expiresAt.split('-').map(Number)
  const [ty, tm, td] = cnToday().split('-').map(Number)
  const days = Math.round((Date.UTC(y, m - 1, d) - Date.UTC(ty, tm - 1, td)) / 86400000)
  if (days >= 0) return false
  return contracts.value.some(o => o.partyId === c.partyId && o.expiresAt !== null && o.expiresAt > c.expiresAt!)
}
// 三类统计单位数（去重）；无到期日合同的单位不计入三卡
const expiryStats = computed(() => {
  const stats: Record<ExpiryCat, number> & { total: number } = { expired: 0, pending: 0, future: 0, total: parties.value.length }
  for (const p of parties.value) {
    const cat = partyLatestCat(p.id)
    if (cat) stats[cat]++
  }
  return stats
})
// 统计卡标签
const EXPIRY_CARDS = [
  { key: 'all', label: '全部' },
  { key: 'expired', label: '已到期' },
  { key: 'pending', label: '即将到期' },
  { key: 'future', label: '未到期' },
] as const

// 打开「合同期至调整」弹窗
function openEditExpiry(c: Contract) {
  editingContract.value = c
  editExpiryDate.value = c.expiresAt || ''
  editExpiryVisible.value = true
}

// 保存「合同期至调整」：空 = 清除到期；保存后刷新列表与角标
async function saveContractExpiry() {
  const c = editingContract.value
  if (!c) return
  const val = editExpiryDate.value || ''
  if (val === (c.expiresAt || '')) {
    editExpiryVisible.value = false
    return
  }
  try {
    await api.put<any>(`/contracts/${c.id}/expiry`, { expiresAt: val || null })
    editExpiryVisible.value = false
    c.expiresAt = val || null
    void refreshContactIssues()
  } catch (e: any) { alert(e?.message || '更新失败') }
}

function openUpload(partyId: number | null) {
  uploadPartyId.value = partyId
  uploadPartyChosen.value = partyId
  chosenFile.value = null
  manualName.value = ''
  contractExpiresAt.value = ''
  dialogVisible.value = true
}

function triggerFile() { fileInputRef.value?.click() }
function isPdfFile(f: File): boolean {
  return f.type === 'application/pdf' || fileExt(f.name).toLowerCase() === 'pdf'
}

function acceptFile(f: File): boolean {
  if (!isPdfFile(f)) { alert('合同只能上传 PDF 格式'); return false }
  return true
}
function onPickFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f && acceptFile(f)) { chosenFile.value = f; manualName.value = cleanContractName(f.name) }
}
function onDrop(e: DragEvent) {
  const f = e.dataTransfer?.files?.[0]
  if (f && acceptFile(f)) { chosenFile.value = f; manualName.value = cleanContractName(f.name) }
}

// 异常文件名的识别与清理：如 "u=4217215850,4193273696&fm=253&fmt=auto&app=138&f=JPEG.jpg"
function isAbnormalName(name: string): boolean {
  const base = name.slice(0, name.lastIndexOf('.'))
  if (/[&=,?]/.test(name)) return true                        // 含 URL 参数残留
  if (!/[\u4e00-\u9fa5a-zA-Z]/.test(base)) return true        // 无任何中/英文（纯数字/乱码）
  if (!base.includes(' ') && base.length > 40) return true    // 超长无空格字符串
  return false
}
const EXT_KIND: Record<string, string> = {
  jpg: '图片合同', jpeg: '图片合同', png: '图片合同', gif: '图片合同', webp: '图片合同', bmp: '图片合同',
  doc: 'Word合同', docx: 'Word合同',
  xls: 'Excel合同', xlsx: 'Excel合同', csv: 'Excel合同',
  pdf: 'PDF合同',
  txt: '文本合同', md: '文本合同',
}
function cleanContractName(raw: string): string {
  if (!isAbnormalName(raw)) return raw.trim()
  const kind = EXT_KIND[fileExt(raw)] || '合同文件'
  return kind
}
// 同单位内避免重复名，追加 (2)(3)
function dedupeContractName(partyId: number, name: string): string {
  const siblings = contracts.value.filter(c => c.partyId === partyId).map(c => c.fileName)
  let n = name, i = 2
  while (siblings.includes(n)) { n = `${name}(${i})`; i++ }
  return n
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

async function submitUpload() {
  const pid = uploadPartyChosen.value
  if (!pid) { alert('请选择所属单位'); return }
  if (!chosenFile.value) { alert('请选择文件'); return }
  if (!isPdfFile(chosenFile.value)) { alert('合同只能上传 PDF 格式'); return }

  try {
    const fileData = await fileToBase64(chosenFile.value)
    const rawName = (manualName.value || '').trim() || cleanContractName(chosenFile.value.name)
    let fileName = rawName
    // 保留扩展名（判断类型/预览用），泛称只替换主名
    const ext = fileExt(chosenFile.value.name)
    if (!fileExt(rawName) && ext) fileName = `${rawName}.${ext}`
    fileName = dedupeContractName(pid, fileName)
    const body = {
      partyId: pid,
      fileName,
      fileSize: chosenFile.value.size,
      mimeType: chosenFile.value.type || 'application/octet-stream',
      contractTitle: fileName,
      fileData,
      expiresAt: contractExpiresAt.value || null,
    }
    await api.post<any>('/contracts', body)
    dialogVisible.value = false
    chosenFile.value = null
    contractExpiresAt.value = ''
    await load()
    void refreshContactIssues()
  } catch (e: any) { alert(e?.message || '上传失败') }
}

async function viewContract(c: Contract) {
  let target: any = c
  if (!c.fileData) {
    try {
      const r: any = await api.get(`/contracts/${c.id}`)
      target = (r?.data ?? r)
    } catch { /* ignore */ }
  }
  if (!target.fileData) { alert('暂无文件内容'); return }

  previewFileName.value = target.fileName
  previewFileData.value = target.fileData
  previewMime.value = target.mimeType || 'application/octet-stream'

  const kind = detectPreviewKind(previewMime.value, previewFileName.value)
  previewKind.value = kind
  previewTextContent.value = ''
  previewHtmlContent.value = ''
  excelSheets.value = []
  excelWorkbook.value = null

  // 同步类型：立即显示
  if (kind === 'iframe' || kind === 'image') {
    previewVisible.value = true
    return
  }

  // 需要异步处理的类型（office-download 尝试提取正文 / word / excel / text）：显示 loading
  previewLoading.value = true
  previewVisible.value = true

  try {
    if (kind === 'office-download') {
      // 老式 .doc：调用后端提取正文文本，成功则切换为文本预览
      const t: any = await api.get(`/contracts/${c.id}/text`)
      const data = t?.data ?? t
      if (data?.supported && data?.text) {
        previewKind.value = 'text'
        previewTextContent.value = data.text
      }
    } else if (kind === 'text') {
      const res = await fetch(target.fileData)
      previewTextContent.value = await res.text()
    } else if (kind === 'word') {
      await renderWord(target.fileData)
    } else if (kind === 'excel') {
      await renderExcel(target.fileData)
    }
  } catch (e: any) {
    alert('文件解析失败：' + (e?.message || '未知错误'))
    previewVisible.value = false
  } finally {
    previewLoading.value = false
  }
}

async function exportZip() {
  // 提示用户正在导出
  const now = new Date()
  const zipName = now.getFullYear() + String(now.getMonth() + 1).padStart(2, '0') + '合同.zip'
  try {
    // 逐份获取完整合同数据（含 fileData）
    const fullList: Contract[] = []
    for (const c of contracts.value) {
      if (c.fileData) {
        fullList.push(c)
      } else {
        try {
          const r = await api.get<any>(`/contracts/${c.id}`)
          fullList.push(r?.data || r)
        } catch { /* 跳过取不到的 */ }
      }
    }
    if (fullList.length === 0) { alert('没有可导出的合同'); return }

    const zip = new JSZip()
    // 按单位分组
    const byParty = new Map<number, Contract[]>()
    for (const c of fullList) {
      if (!byParty.has(c.partyId)) byParty.set(c.partyId, [])
      byParty.get(c.partyId)!.push(c)
    }
    for (const [partyId, files] of byParty) {
      const pName = partyName(partyId).replace(/[/\\?*:<>|"]/g, '_') // 去除非法字符
      for (const f of files) {
        if (f.fileData) {
          const commaIdx = f.fileData.indexOf(',')
          const base64 = commaIdx >= 0 ? f.fileData.substring(commaIdx + 1) : f.fileData
          zip.file(pName + '/' + f.fileName, base64, { base64: true })
        }
      }
    }
    const blob = await zip.generateAsync({ type: 'blob' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = zipName; a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) {
    alert('导出失败：' + (e?.message || '未知错误'))
  }
}

function remove(c: Contract) {
  pendingDelete.value = c
  delCountdown.value = 5
  delVisible.value = true
  if (delTimer !== null) clearInterval(delTimer)
  delTimer = window.setInterval(() => {
    delCountdown.value--
    if (delCountdown.value <= 0) {
      if (delTimer !== null) clearInterval(delTimer)
      delTimer = null
    }
  }, 1000)
}

// 二次验证弹窗「确定删除」：倒计时归零后才可点击
function confirmDelete() {
  const c = pendingDelete.value
  if (!c) return
  api.del(`/contracts/${c.id}`).then(() => {
    delVisible.value = false
    pendingDelete.value = null
    void load()
    void refreshContactIssues() // 删除后刷新角标，去除对应的到期提醒
  }).catch((e: any) => alert(e?.message || '删除失败'))
}

function closeDeleteDialog() {
  delVisible.value = false
  pendingDelete.value = null
  if (delTimer !== null) clearInterval(delTimer)
  delTimer = null
}

// 全局 Escape 关闭预览
function handleEscape(e: KeyboardEvent) {
  if (e.key === 'Escape' && previewVisible.value) {
    previewVisible.value = false
  }
}
onMounted(() => {
  load()
  window.addEventListener('keydown', handleEscape)
})
onUnmounted(() => {
  window.removeEventListener('keydown', handleEscape)
  if (delTimer !== null) clearInterval(delTimer)
})
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: #969799; margin: 4px 0 0; }
.cp-header-actions { display: flex; gap: 8px; align-items: center; }

.cp-filters { display: flex; gap: 10px; margin-bottom: 12px; align-items: center; flex-wrap: wrap; }
.cp-search { flex: 1; min-width: 180px; }
.cp-select {
  padding: 6px 10px; border-radius: 6px; border: 1px solid #eaeaea; font-size: 13px;
  background: #fff; color: #1f2329; outline: none;
}

.cp-group {
  background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 10px;
  margin-bottom: 12px; overflow: hidden;
}
.cp-group-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 14px; background: #fafbfc; border-bottom: 1px solid var(--line-soft, #eaeaea);
  cursor: pointer; user-select: none;
}
.cp-group-left { display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; }
.cp-chev { font-size: 11px; color: #969799; width: 14px; }
.cp-party-name { font-weight: 600; color: #1f2329; }
.cp-party-type { font-size: 11px; padding: 1px 7px; border-radius: 10px; }
.tag-invest   { background: #e6f1ff; color: var(--info); }
.tag-reinvest { background: #f0e6ff; color: var(--accent); }
.tag-flow     { background: #fff2e6; color: var(--warn); }
.tag-other    { background: #f2f3f5; color: #646566; }
.cp-count { font-size: 12px; color: #969799; }

.cp-group-body { padding: 8px 12px 14px; }
.cp-empty-group { text-align: center; color: #969799; font-size: 12px; padding: 12px 0; }
.cp-file-item {
  display: flex; align-items: center; gap: 12px;
  padding: 10px; border-radius: 8px; margin-bottom: 6px;
  background: #fafbfc;
}
.cp-file-ic {
  width: 36px; height: 36px; border-radius: 8px;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 16px; color: #fff; flex-shrink: 0;
  background: var(--info);
}
.cp-file-ic.ic-doc, .cp-file-ic.ic-docx { background: #2b579a; }
.cp-file-ic.ic-pdf { background: #d4380d; }
.cp-file-ic.ic-jpg, .cp-file-ic.ic-jpeg, .cp-file-ic.ic-png, .cp-file-ic.ic-gif { background: #52c41a; }
.cp-file-ic.ic-archived { background: #c8c9cc; }
.cp-file-info { flex: 1; min-width: 0; }
.cp-file-name { font-weight: 500; color: #1f2329; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
.cp-file-name:hover { color: var(--info); }
.cp-file-name.cp-file-archived { color: #c8c9cc; font-weight: 400; }
.cp-file-meta { font-size: 11px; color: #969799; margin-top: 2px; }
.cp-file-actions { display: flex; gap: 6px; flex-shrink: 0; }
.empty { padding: 40px 0; }

/* 合同预警统计卡（参照投资收益统计卡：圆角8px + 四色淡彩背景） */
.cp-stats {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 14px;
}
.cp-stat-card {
  background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px;
  padding: 10px 12px; cursor: pointer; transition: box-shadow .15s;
}
.cp-stat-label { font-size: 12px; color: #969799; }
.cp-stat-num { font-size: 16px; font-weight: 600; color: var(--ink-900, #1f2329); margin-top: 2px; font-variant-numeric: tabular-nums; }
.cp-stats .cp-stat-card { border-color: transparent; }
.cp-stats .cp-stat-card:nth-child(1) { background: #e8f0fb; }
.cp-stats .cp-stat-card:nth-child(2) { background: #e6f5f4; }
.cp-stats .cp-stat-card:nth-child(3) { background: #f1edfc; }
.cp-stats .cp-stat-card:nth-child(4) { background: #fff1e0; }
/* 选中态：深描边区分，保持淡彩背景 */
.cp-stat-active { box-shadow: inset 0 0 0 1.5px rgba(31, 35, 41, .35); }
.cp-stat-active .cp-stat-num { font-weight: 700; color: #1f2329; }
.cp-stat-active .cp-stat-label { color: #646566; }

.up-body { padding: 10px 4px; min-width: 380px; }
.up-field { margin-bottom: 16px; }
.up-field > label { display: block; font-size: 12px; color: #646566; margin-bottom: 6px; }
.up-party-fixed { padding: 8px 12px; background: #f7f8fa; border-radius: 6px; font-size: 13px; color: #1f2329; }
.up-select { width: 100%; padding: 8px 12px; border: 1px solid #dcdfe6; border-radius: 6px; font-size: 13px; color: #1f2329; background: #fff; outline: none; }
.up-select:focus { border-color: var(--info); }
.up-name-input { width: 100%; box-sizing: border-box; padding: 8px 12px; border: 1px solid #dcdfe6; border-radius: 6px; font-size: 13px; color: #1f2329; background: #fff; outline: none; }
.up-name-input:focus { border-color: var(--info); }
.up-hint { font-size: 11px; color: #969799; margin-top: 4px; }

/* 单位行到期状态标签（跟单位，按最新期至） */
.party-expiry-tag {
  display: inline-block; margin-left: 8px; padding: 1px 7px;
  border-radius: 8px; font-size: 11px; font-weight: 500; white-space: nowrap;
  background: #ebedf0; color: #969799;
}
.party-expiry-tag.pet-expired { background: #ffece8; color: var(--danger); font-weight: 600; }
.party-expiry-tag.pet-pending { background: #fff7e6; color: #ed6a0c; font-weight: 600; }
.party-expiry-tag.pet-ok { background: #e8f9f5; color: var(--success); }

/* 合同期至调整弹窗 */
.ee-body { padding: 12px 8px; min-width: 260px; }
.ee-sub { font-size: 12px; color: #969799; margin-bottom: 10px; word-break: break-all; }
.ee-date { width: 100%; box-sizing: border-box; padding: 8px 12px; border: 1px solid #dcdfe6; border-radius: 6px; font-size: 13px; color: #1f2329; background: #fff; outline: none; }
.ee-date:focus { border-color: var(--info); }
.ee-tip { font-size: 11px; color: #969799; margin-top: 6px; }

/* 删除二次验证弹窗 */
/* 单个合同到期展示 */
.cp-file-expiry { display: flex; align-items: center; gap: 6px; font-size: 12px; margin-top: 2px; }
.cfe-label { color: #969799; }
.cfe-date { color: #1f2329; }
.cfe-state { color: #646566; }
.cp-file-expiry.ce-none .cfe-date { color: #c8c9cc; }
.cp-file-expiry.ce-warn .cfe-date,
.cp-file-expiry.ce-warn .cfe-state { color: var(--danger); font-weight: 600; }
.dc-body { padding: 12px 16px 4px; text-align: center; }
.dc-file { font-size: 13px; font-weight: 600; color: #1f2329; word-break: break-all; margin-bottom: 8px; }
.dc-warn { font-size: 13px; color: var(--danger); line-height: 1.5; }
.dc-count { font-size: 12px; color: #969799; margin-top: 10px; }
.dc-btns { display: flex; justify-content: center; gap: 12px; margin-top: 16px; }
.up-drop {
  border: 2px dashed #dcdee0; border-radius: 10px; padding: 22px 16px;
  text-align: center; color: #969799; cursor: pointer; transition: all .15s;
}
.up-drop:hover { border-color: var(--jade, #07c160); color: var(--jade, #07c160); }
.up-drop-ic { font-size: 28px; margin-bottom: 6px; }
.up-drop-sub { font-size: 11px; margin-top: 4px; }
.up-chosen { display: flex; align-items: center; justify-content: space-between; color: #1f2329; font-weight: 500; gap: 12px; }

/* 预览弹窗（全屏样式在下方非 scoped 块中） */
.preview-wrap { display: flex; flex-direction: column; flex: 1; overflow: hidden; }
.preview-loading {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 16px; color: #646566;
}
.preview-text {
  flex: 1;
  padding: 20px 28px; margin: 0;
  background: #fff; font-size: 14px; line-height: 1.7;
  white-space: pre-wrap; word-break: break-all;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  overflow: auto;
}
.preview-img { flex: 1; display: block; object-fit: contain; background: #2b2b2b; }
.preview-iframe { flex: 1; width: 100%; height: 100%; border: none; background: #fff; }
.preview-empty { padding: 40px; text-align: center; color: #969799; }

/* Word 渲染样式（mammoth 转出来的 HTML） */
.preview-docx {
  flex: 1; overflow: auto;
  padding: 32px 48px; background: #fff;
  font-size: 14px; line-height: 1.8; color: #1f2329;
}
.preview-docx :deep(h1) { font-size: 22px; margin: 16px 0 8px; font-weight: 700; }
.preview-docx :deep(h2) { font-size: 18px; margin: 14px 0 6px; font-weight: 700; }
.preview-docx :deep(h3) { font-size: 16px; margin: 12px 0 6px; font-weight: 600; }
.preview-docx :deep(p)  { margin: 6px 0; }
.preview-docx :deep(ul), .preview-docx :deep(ol) { padding-left: 24px; margin: 6px 0; }
.preview-docx :deep(table) { border-collapse: collapse; margin: 8px 0; width: 100%; }
.preview-docx :deep(td), .preview-docx :deep(th) { border: 1px solid #d0d0d0; padding: 6px 10px; }
.preview-docx :deep(img) { max-width: 100%; height: auto; }

/* Excel 样式 */
.preview-excel { flex: 1; display: flex; flex-direction: column; background: #fff; overflow: hidden; }
.excel-sheet-tabs { display: flex; gap: 4px; padding: 6px 10px; border-bottom: 1px solid #ebedf0; background: #f7f8fa; flex-shrink: 0; }
.excel-tab {
  padding: 4px 12px; border: 1px solid #dcdfe6; background: #fff; border-radius: 4px;
  font-size: 12px; color: #646566; cursor: pointer;
}
.excel-tab.active { background: var(--info); color: #fff; border-color: var(--info); }
.excel-scroll { flex: 1; overflow: auto; padding: 12px; background: #fafbfc; }
.excel-table { border-collapse: collapse; font-size: 12px; background: #fff; }
.excel-table td, .excel-table th { border: 1px solid #d0d0d0; padding: 4px 8px; white-space: nowrap; }
.excel-table th { background: #f7f8fa; font-weight: 600; }

/* 老式 Office 下载提示 */
.preview-office-tip {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 40px; background: #f7f8fa; text-align: center;
}
.preview-office-tip .office-ic { font-size: 64px; margin-bottom: 24px; }
.preview-office-tip p { margin: 0; font-size: 16px; color: #1f2329; }
.preview-office-tip .office-sub { font-size: 13px; color: #969799; margin-top: 8px; }
.download-btn {
  display: inline-block; margin-top: 28px; padding: 12px 32px;
  background: var(--info); color: #fff; font-size: 14px;
  text-decoration: none; border-radius: 6px;
  transition: background .15s;
}
.download-btn:hover { background: #127be6; }
</style>

<!-- 预览弹窗是非 scoped 的，因为 preview-dialog 被 Teleport 到 body 上，scoped 对它不起作用 -->
<style>
/* 手写全屏预览弹窗样式（van-dialog 已废弃，Teleport 到 body） */
.preview-dialog {
  position: fixed !important;
  top: 0 !important; left: 0 !important; right: 0 !important; bottom: 0 !important;
  width: 100vw !important;
  height: 100vh !important;
  background: #fff !important;
  z-index: 99999 !important;
  display: flex !important;
  flex-direction: column !important;
}
.preview-header {
  flex-shrink: 0;
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 20px;
  background: #fff;
  border-bottom: 1px solid #ebedf0;
  box-shadow: 0 1px 2px rgba(0,0,0,0.04);
}
.preview-title {
  font-size: 15px; font-weight: 600; color: #1f2329;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 70%;
}
.preview-close-btn {
  padding: 6px 18px; font-size: 13px;
  background: #fff; border: 1px solid #dcdfe6; border-radius: 4px;
  color: #646566; cursor: pointer; transition: all .15s;
}
.preview-close-btn:hover { background: #f5f7fa; border-color: var(--info); color: var(--info); }

.preview-body {
  flex: 1 !important;
  display: flex !important;
  flex-direction: column !important;
  overflow: hidden !important;
  background: #fff;
}
.preview-loading {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 16px; color: #646566;
}
</style>
