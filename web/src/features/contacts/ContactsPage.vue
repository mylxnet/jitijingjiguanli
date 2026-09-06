<template>
  <div class="contacts-page">
    <div class="page-header">
      <h3>往来</h3>
      <div class="header-actions">
        <van-button type="primary" plain size="small" @click="openAccrue">年度结转</van-button>
        <van-button type="primary" plain size="small" icon="down" @click="openExportDialog">导出</van-button>
        <van-button type="primary" size="small" icon="plus" @click="openAddParty">新增单位</van-button>
      </div>
    </div>

    <!-- 欠款分类 banner -->
    <div class="owe-banners">
      <div class="owe-banner total" :class="{ active: activeTypeFilter === 'all' }" @click="toggleBanner('all')">
        <div class="banner-label">欠款总计</div>
        <div class="banner-value">{{ formatFen(totalOwe) }}</div>
      </div>
      <div class="owe-banner rent" :class="{ active: activeTypeFilter === 'flow' }" @click="toggleBanner('flow')">
        <div class="banner-label">流转费欠款</div>
        <div class="banner-value">{{ formatFen(rentOweTotal) }}</div>
      </div>
      <div class="owe-banner invest" :class="{ active: activeTypeFilter === 'invest' }" @click="toggleBanner('invest')">
        <div class="banner-label">投资收益欠款</div>
        <div class="banner-value">{{ formatFen(dividendOweTotal) }}</div>
      </div>
    </div>

    <!-- 列表：往来单位 -->
    <div v-if="loading" class="loading-state"><van-skeleton title :row="4" /></div>
    <div v-else-if="parties.length === 0" class="empty-state">
      <p>还没有往来单位</p>
      <van-button size="small" type="primary" @click="openAddParty">新增单位</van-button>
    </div>
    <div v-else>
      <div v-if="displayParties.length === 0" class="empty-state">
        <p>{{ activeTypeFilter === 'flow' ? '暂无流转企业' : activeTypeFilter === 'invest' ? '暂无投资公司' : '还没有往来单位' }}</p>
      </div>
      <div v-else class="party-list">
        <div v-for="p in displayParties" :key="p.id" class="party-row" @click="openDetail(p)">
          <div class="party-main">
            <span class="party-name">{{ p.name }}</span>
            <span class="l2-chip" :class="p.type">{{ partyTypeLabel[p.type ?? 'flow'] || '流转企业' }}</span>
            <span v-if="p.type === 'invest'" class="l2-chip invest-chip">投资 {{ formatFen(p.investAmountCents || 0) }}</span>
          </div>
          <div class="party-owed">
            <div class="owed-value" :class="{ zero: p.outstandingCents <= 0 }">
              {{ formatFen(p.outstandingCents) }}
            </div>
            <div class="owed-label">欠款</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 单位详情弹窗 -->
    <van-popup
      v-model:show="showPartyDetail"
      :position="popupPos()"
      round
      closeable
      class="party-detail-popup"
      @closed="onPartyDetailClosed"
    >
      <div v-if="currentParty">
        <div class="party-detail-head">
          <div class="pd-title">
            <h3 class="detail-title">{{ currentParty.name }}</h3>
            <span v-if="currentParty.type" class="pd-type-text">{{ partyTypeLabel[currentParty.type] || '' }}</span>
          </div>
        </div>

        <div class="detail-block">
          <div class="section-title">
            基本情况
          </div>
          <div class="party-summary">
          <div class="summary-item">
            <div class="summary-value" :class="{ zero: currentParty.outstandingCents <= 0 }">
              {{ formatFen(currentParty.outstandingCents) }}
            </div>
            <div class="summary-label">合计欠款</div>
          </div>
          <div class="summary-item">
            <div class="summary-value">{{ receivableItems.length }}</div>
            <div class="summary-label">应收单</div>
          </div>
          <div class="summary-meta">
            <span class="meta-line">联系电话：{{ currentParty.contactPhone || '—' }}</span>
            <span v-if="currentParty.type === 'flow'" class="meta-line">流转面积：{{ currentParty.areaMu || 0 }} 亩</span>
            <span v-if="currentParty.note" class="meta-line">备注：{{ currentParty.note }}</span>
            <span v-if="currentParty.type === 'invest'" class="meta-line">投资金额（只读）{{ formatFen(currentParty.investAmountCents || 0) }} = 长期投资同名公司累计投出</span>
            <span v-if="currentParty.type === 'flow'" class="meta-line">
              年度流转费标准：{{ currentPartyStd ? formatFen(currentPartyStd.amountCents) : '未设置' }}
              <a class="std-edit-link" @click="openStdForParty">去修改</a>
            </span>
            <span v-if="currentParty.type === 'invest'" class="meta-line">
              年度应得分红：{{ currentPartyStd ? formatFen(currentPartyStd.amountCents) : '未设置' }}
              <a class="std-edit-link" @click="openStdForParty">去修改</a>
            </span>
          </div>
          </div>
        </div>

        <!-- 合同附件 -->
        <div class="detail-block">
          <div class="section-title">
            合同附件
            <span class="section-subtitle">（{{ contracts.length }} 个）</span>
          </div>
          <div v-if="contracts.length === 0" class="empty-state">
            <p>暂无合同或附件</p>
          </div>
          <div v-else class="contract-list">
            <div v-for="c in contracts" :key="c.id" class="contract-item">
              <van-icon :name="contractIcon(c.fileName, c.mimeType)" class="contract-icon" />
              <div class="contract-info" @click="previewContract(c)">
                <div class="contract-name">{{ displayContractName(c) }}</div>
                <div class="contract-meta">
                  <span>{{ displayContractFileName(c) }}</span>
                  <span>{{ formatFileSize(c.fileSize) }}</span>
                  <span v-if="c.contractDate">签订 {{ c.contractDate }}</span>
                </div>
              </div>
              <van-button size="mini" plain type="primary" @click.stop="previewContract(c)">查看</van-button>
              <van-button size="mini" plain type="danger" @click.stop="deleteContract(c)">删除</van-button>
            </div>
          </div>

          <!-- 上传区 -->
          <div class="contract-upload">
            <input
              type="file"
              :id="'contract-upload-' + (currentParty?.id ?? 0)"
              class="hidden-file-input"
              @change="async (e) => { const f = (e.target as HTMLInputElement).files?.[0]; if (f) await onContractUpload(f); (e.target as HTMLInputElement).value = '' }"
            />
            <label :for="'contract-upload-' + (currentParty?.id ?? 0)" class="upload-trigger">
              <van-icon name="plus" />
              <span>{{ contractUploading ? '上传中…' : '上传合同/附件' }}</span>
            </label>
            <div class="upload-hint">支持 PDF / 图片 / Word / Excel，单个 ≤ 5MB</div>
          </div>
        </div>

        <div class="detail-block">
          <div class="section-title">应收记录</div>
          <div v-if="detailLoading" class="loading-state"><van-skeleton title :row="4" /></div>
        <div v-else-if="receivableItems.length === 0" class="empty-state">
          <p>该单位暂无应收记录</p>
        </div>

        <div v-else class="recv-list">
          <div v-for="r in receivableItems" :key="r.id" class="recv-card">
            <div class="recv-head" @click="toggleReceipts(r)">
              <div class="recv-title-wrap">
                <span class="recv-title">{{ r.title }}</span>
                <span class="l2-chip" :class="r.recvKind">{{ recvKindLabel[r.recvKind] }}</span>
                <span class="l2-chip" :class="r.status">{{ r.status === 'open' ? '未结清' : '已结清' }}</span>
              </div>
              <van-icon :name="expandedReceipts[r.id] ? 'arrow-up' : 'arrow-down'" />
            </div>
            <div class="recv-amounts">
              <div class="amount-cell"><span class="amount-label">应收</span>{{ formatFen(r.amountCents) }}</div>
              <div class="amount-cell"><span class="amount-label">已收</span>{{ formatFen(r.paidCents) }}</div>
              <div class="amount-cell strong"><span class="amount-label">未收</span>{{ formatFen(r.outstandingCents) }}</div>
            </div>
            <div class="recv-actions">
              <van-button v-if="r.status === 'open' && r.paidCents === 0" size="mini" plain type="danger" @click="voidReceivable(r)">作废</van-button>
              <van-button v-if="r.status === 'open'" size="mini" plain type="primary" @click="openReceipt(r)">收款 / 抵销</van-button>
            </div>

            <!-- 核销记录 -->
            <div v-if="expandedReceipts[r.id]" class="receipt-list">
              <div v-if="!receiptsByRec[r.id] || receiptsByRec[r.id].length === 0" class="receipt-empty">暂无核销记录</div>
              <div v-for="rc in receiptsByRec[r.id] || []" :key="rc.id" class="receipt-row" :class="{ voided: rc.status === 'voided' }">
                <span class="receipt-method" :class="rc.method">{{ rc.method === 'cash' ? '现金' : '抵销' }}</span>
                <span class="receipt-amount">{{ rc.method === 'cash' ? '+' : '抵' }}{{ formatFen(rc.amountCents) }}</span>
                <span class="receipt-date">{{ rc.receiptDate }}</span>
                <span v-if="rc.status === 'voided'" class="l2-chip stopped">已作废</span>
                <van-button
                  v-if="rc.status === 'normal'"
                  size="mini"
                  plain
                  type="warning"
                  @click="voidReceipt(rc)"
                >作废</van-button>
              </div>
            </div>
          </div>
        </div>
        </div>
      </div>
    </van-popup>

    <!-- 导出筛选 -->
    <van-dialog v-model:show="showExportDialog" title="导出往来单位及应收汇总" show-cancel-button @confirm="doExport">
      <van-form>
        <van-cell-group inset>
          <van-field label="单位类型">
            <template #input>
              <van-checkbox-group v-model="exportFilter.types" shape="square">
                <van-checkbox name="flow">土地流转企业</van-checkbox>
                <van-checkbox name="invest">长期投资单位</van-checkbox>
                <van-checkbox name="reinvest">再投资单位</van-checkbox>
                <van-checkbox name="other">其他单位</van-checkbox>
              </van-checkbox-group>
            </template>
          </van-field>
          <van-field label="应收状况">
            <template #input>
              <van-radio-group v-model="exportFilter.recvState" direction="horizontal">
                <van-radio name="all">全部</van-radio>
                <van-radio name="has">有应收</van-radio>
                <van-radio name="none">无应收</van-radio>
              </van-radio-group>
            </template>
          </van-field>
          <van-cell title="预计导出数量" :value="exportPreviewCount + ' 个单位'" />
        </van-cell-group>
      </van-form>
    </van-dialog>

    <!-- 新增/编辑往来单位 -->
    <van-dialog v-model:show="showAddParty" :title="editParty ? '编辑往来单位' : '新增往来单位'" show-cancel-button @confirm="saveParty" class="party-edit-dialog">
      <van-form>
        <van-tabs v-model:active="formTab" sticky shrink class="party-form-tabs">
          <van-tab title="基本情况" name="basic">
            <van-cell-group inset>
              <van-field v-model="partyForm.name" label-width="150" label="单位名称" placeholder="如：XX公司 / XX合作社" required />
              <van-field label="单位类型">
                <template #input>
                  <van-radio-group v-model="partyForm.type" direction="horizontal">
                    <van-radio name="invest">长期投资</van-radio>
                    <van-radio name="reinvest">再投资</van-radio>
                    <van-radio name="flow">土地流转</van-radio>
                    <van-radio name="other">其他</van-radio>
                  </van-radio-group>
                </template>
              </van-field>
              <van-field v-model="partyForm.contactPhone" label-width="150" label="联系电话" placeholder="可选" />
            </van-cell-group>

            <van-cell-group inset title="合同与备注">
              <div class="inline-upload">
                <input
                  type="file"
                  :id="'inline-file-' + (currentParty?.id ?? 'new')"
                  class="hidden-file-input"
                  @change="handleInlineUpload"
                />
                <label :for="'inline-file-' + (currentParty?.id ?? 'new')" class="upload-trigger inline">
                  <van-icon name="plus" />
                  <span>{{ inlineUploading ? '上传中…' : '点击选择合同/附件' }}</span>
                </label>
                <div v-if="inlineContracts.length > 0" class="inline-contract-list">
                  <div v-for="(c, i) in inlineContracts" :key="i" class="inline-contract-item">
                    <van-icon :name="contractIcon(c.fileName, c.mimeType)" class="contract-icon" />
                    <span class="inline-ctitle">{{ splitCleanFileName(c.fileName).cleanName + splitCleanFileName(c.fileName).ext }}</span>
                    <span class="inline-cstate" v-if="c._status === 'uploading'">上传中…</span>
                    <span class="inline-cstate ok" v-else>✓</span>
                    <van-icon name="cross" class="inline-cremove" @click="inlineContracts.splice(i, 1)" />
                  </div>
                </div>
              </div>
              <van-field v-model="partyForm.note" label-width="150" label="备注" placeholder="备注（可选）" />
            </van-cell-group>
          </van-tab>

          <van-tab title="年度数据" name="data">
            <!-- 投资/再投资专属字段 -->
            <van-cell-group inset v-if="partyForm.type === 'invest' || partyForm.type === 'reinvest'" title="投资信息">
              <van-field v-model="partyForm.investAmountYuan" label-width="150" type="number" label="投资本金（元）" placeholder="如 500000" inputmode="decimal" />
              <van-field v-model="partyForm.returnRatePercent" label-width="150" type="number" label="年收益率" placeholder="如 0.04" inputmode="decimal" />
              <van-field v-model="partyForm.expectedReturnYuan" label-width="150" type="number" label="年收益（元）" placeholder="自动计算，可修改" inputmode="decimal" />
            </van-cell-group>

            <!-- 土地流转专属字段 -->
            <van-cell-group inset v-else-if="partyForm.type === 'flow'" title="土地流转信息">
              <van-field v-model="partyForm.landMuYuan" label-width="150" type="number" label="流转亩数" placeholder="如 200" inputmode="decimal" />
              <van-field v-model="partyForm.landFeePerMuYuan" label-width="150" type="number" label="每亩年流转费" placeholder="如 150" inputmode="decimal" />
              <van-field v-model="partyForm.expectedLandFeeYuan" label-width="150" type="number" label="总流转费（元）" placeholder="自动计算，可修改" inputmode="decimal" />
              <van-field v-model="partyForm.mgmtFeePerMuYuan" label-width="150" type="number" label="每亩年管理费" placeholder="如 15" inputmode="decimal" />
              <van-field v-model="partyForm.expectedMgmtFeeYuan" label-width="150" type="number" label="总管理费（元）" placeholder="自动计算，可修改" inputmode="decimal" />
            </van-cell-group>

            <van-cell-group inset v-else title="其他类型">
              <van-field label="说明" placeholder="可选" readonly value="该单位不参与年度投资/流转费结转，需要时手动登记应收" />
            </van-cell-group>
          </van-tab>
        </van-tabs>
      </van-form>
    </van-dialog>

    <!-- 年度标准就地修改 -->
    <van-dialog v-model:show="showStdEditDialog" :title="stdEditTitle" show-cancel-button @confirm="saveStdFromDetail">
      <van-field v-model="stdEditYuan" type="number" :label="stdEditLabel" placeholder="0.00" inputmode="decimal" />
      <div class="dialog-tip">保存后作为该单位年度结转的标准金额</div>
    </van-dialog>

    <!-- 批量计提 / 流转费标准 -->
    <van-popup v-model:show="showAccrue" :position="popupPos()" round closeable style="max-height: 92vh">
      <div class="accrue-popup">
        <van-tabs v-model:active="accrueTab">
          <!-- 年度结转：预览单位标准数据后再确认 -->
          <van-tab title="年度结转" name="accrue">
            <div class="accrue-body">
              <van-field v-model="accrueYear" type="digit" label="年度" placeholder="如 2026" />
              <div class="accrue-save">
                <van-button round block type="primary" plain :loading="previewLoading" @click="previewAccrue">生成预览（从各单位年度标准带数据）</van-button>
              </div>
              <div v-if="previewItems.length > 0" class="preview-list">
                <div v-for="it in previewItems" :key="it.kind + '-' + it.partyId" class="std-row">
                  <span class="std-name">{{ it.partyName }}（{{ it.kind === 'rent' ? '流转费' : '投资收益' }}）</span>
                  <span class="std-amount">{{ formatFen(it.amountCents) }}</span>
                  <span class="preview-state" :class="{ dup: it.exists }">{{ it.exists ? '已存在跳过' : '将新增' }}</span>
                </div>
                <div v-if="previewItems.length === 0" class="accrue-empty">该年度没有可结转的标准数据</div>
              </div>
              <div v-else-if="previewLoaded" class="accrue-empty">该年度没有可结转的标准数据（先到单位资料/年度标准里设好金额）</div>
              <div class="accrue-save">
                <van-button
                  round block type="primary" :loading="accruing" :disabled="previewNewCount === 0"
                  @click="accrueNow"
                >确认结转（将新增 {{ previewNewCount }} 条）</van-button>
              </div>
            </div>
          </van-tab>

          <!-- 年度标准 + 一键结转（按单位类型：流转费 / 投资收益） -->
          <van-tab title="年度标准" name="std">
            <div class="accrue-body">
              <div class="std-form">
                <van-field
                  :model-value="stdForm.partyName || '请选择单位'"
                  is-link readonly label="单位"
                  @click="openAccrueUnit('std', 0)"
                  class="only-mobile"
                />
                <NativeSelect
                  label="单位"
                  placeholder="请选择单位"
                  v-model="stdForm.partyId"
                  :options="accrueUnitActions"
                />
                <van-field v-model="stdForm.amountYuan" type="number" :label="stdAmountLabel" placeholder="0.00" inputmode="decimal" />
                <div v-if="stdKindError" class="dialog-tip">该单位是“其它单位”，不参与年度结转；需要时手动登记应收</div>
                <van-button size="small" round block type="primary" :loading="savingStd" @click="saveStandard">保存标准</van-button>
              </div>
              <div class="std-list">
                <div v-for="s in standards" :key="s.id" class="std-row">
                  <span class="std-name">{{ s.partyName }}（{{ s.recvKind === 'dividend' ? '年度分红' : '年度流转费' }}）</span>
                  <span class="std-amount">{{ formatFen(s.amountCents) }}</span>
                  <van-switch :model-value="s.active" size="20" @update:model-value="(v:boolean) => toggleStandard(s, v)" />
                </div>
                <div v-if="standards.length === 0" class="accrue-empty">还没有年度标准，先在上方添加</div>
              </div>
              <div class="accrue-save">
                <van-button round block type="primary" :loading="accruing" @click="accrueNow">
                  一键结转 {{ accrueYear }} 年度（流转费+投资收益）
                </van-button>
              </div>
            </div>
          </van-tab>
        </van-tabs>
      </div>
    </van-popup>

    <!-- 批量计提/标准：单位选择 -->
    <van-action-sheet
      v-model:show="showAccrueUnitPicker"
      title="选择单位"
      :actions="accrueUnitActions"
      @select="onAccrueUnitSelect"
      @cancel="showAccrueUnitPicker = false"
    />

    <!-- 登记应收 -->
    <van-dialog v-model:show="showAddReceivable" title="登记应收" show-cancel-button @confirm="saveReceivable">
      <van-field v-if="currentParty" :model-value="currentParty.name" label="单位" readonly />
      <van-field label="类别">
        <template #input>
          <van-radio-group v-model="recvForm.recvKind" direction="horizontal">
            <van-radio name="rent">流转费</van-radio>
            <van-radio name="dividend">投资收益</van-radio>
            <van-radio name="other">其他</van-radio>
          </van-radio-group>
        </template>
      </van-field>
      <van-field v-model="recvForm.title" label="事由" placeholder="如：2026年度土地流转费" :rules="[{ required: true }]" />
      <van-field v-model="recvForm.amount" label="应收金额" type="number" placeholder="0.00" inputmode="decimal" />
      <van-field v-model="recvForm.note" label="备注" placeholder="备注（可选）" />
    </van-dialog>

    <!-- 收款 / 抵销 -->
    <van-popup v-model:show="showReceipt" :position="popupPos()" round closeable style="max-height: 92vh">
      <div class="receipt-popup">
        <div class="popup-title">收款 / 抵销</div>
        <van-field label="应收单">
          <template #input>
            <div class="static-text">
              {{ currentReceivable?.title }}（未收 {{ currentReceivable ? formatFen(currentReceivable.outstandingCents) : '' }}）
            </div>
          </template>
        </van-field>
        <van-field label="方式">
          <template #input>
            <van-radio-group v-model="receiptForm.method" direction="horizontal">
              <van-radio name="cash">现金入账</van-radio>
              <van-radio name="offset">抵销</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field v-model="receiptForm.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
        <van-field v-model="receiptForm.date" label="日期" placeholder="YYYY-MM-DD" />
        <van-field v-if="receiptForm.method === 'offset'" class="only-mobile">
          <template #label>抵销流水</template>
          <template #input>
            <div class="static-text" v-if="receiptForm.txnLabel">{{ receiptForm.txnLabel }}</div>
            <van-button v-else size="mini" type="primary" plain @click="loadOffsetTxns">选择分红支出流水</van-button>
          </template>
        </van-field>
        <NativeSelect
          v-if="receiptForm.method === 'offset'"
          label="抵销流水"
          placeholder="选择分红支出流水"
          :model-value="receiptForm.txnId"
          :options="offsetTxnOptions"
          @update:model-value="(v:number|string|null) => { receiptForm.txnId = (v == null ? null : Number(v)); const o=offsetTxnOptions.find(x=>x.value===Number(v)); receiptForm.txnLabel = o ? o.name : ''; }"
        />
        <div v-if="receiptForm.method === 'cash'" class="dialog-tip">
          现金收款将自动记一笔银行收入
          <span v-if="cashCategoryResolved">{{ cashCategoryResolved }}</span>
          <template v-else>（需要选择入账科目）</template>
        </div>
        <van-field
          v-if="receiptForm.method === 'cash'"
          :model-value="receiptForm.categoryName || '请选择入账科目'"
          is-link
          readonly
          label="入账科目"
          @click="openIncomeCatPicker()"
          class="only-mobile"
        />
        <NativeSelect
          v-if="receiptForm.method === 'cash'"
          label="入账科目"
          placeholder="请选择入账科目"
          :model-value="receiptForm.categoryId"
          :options="incomeCatOptions"
          @update:model-value="(v:number|string|null) => { receiptForm.categoryId = (v == null ? null : Number(v)); const o=incomeCatOptions.find(x=>x.value===Number(v)); receiptForm.categoryName = o ? o.name : ''; }"
        />
        <van-field v-model="receiptForm.note" label="备注" placeholder="备注（可选）" />
        <div class="receipt-save">
          <van-button round block type="primary" :disabled="!canSubmitReceipt" :loading="savingReceipt" @click="saveReceipt">
            保存核销
          </van-button>
        </div>
      </div>
    </van-popup>

    <!-- 入账科目选择器 -->
    <van-action-sheet
      v-model:show="showIncomeCatPicker"
      title="选择入账科目"
      :actions="incomeCatOptions"
      @select="onIncomeCatSelect"
      @cancel="showIncomeCatPicker = false"
    />
    <!-- 抵销流水选择器 -->
    <van-action-sheet
      v-model:show="showTxnPicker"
      title="选择分红支出流水"
      :actions="offsetTxnOptions"
      @select="onTxnSelect"
      @cancel="showTxnPicker = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '../../lib/http'
import { showToast, showDialog } from 'vant'
import type {
   Category, Party, Receivable, Receipt, ReceivableDetail,
   ReceivableListResponse, Transaction, ApiResponse, RecvKind, AccrualStandard, AccrueResult, AccruePreview, AccruePreviewItem,
   Contract, ReinvestAllocation,
 } from '../../types/api'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import NativeSelect from '../../components/NativeSelect.vue'
import * as XLSX from 'xlsx'
import { popupPos } from '../../composables/useScreen';
const loading = ref(false)
const detailLoading = ref(false)
const parties = ref<Party[]>([])

// ---- 往来导出（纯前端 xlsx） ----
const showExportDialog = ref(false)
interface ExportFilter {
  types: string[]
  recvState: 'all' | 'has' | 'none'
}
const exportFilter = ref<ExportFilter>({ types: ['flow', 'invest', 'reinvest', 'other'], recvState: 'all' })

const exportPreviewCount = computed(() => applyExportFilter(parties.value).length)

function openExportDialog() {
  // 默认全选所有类型
  exportFilter.value = { types: ['flow', 'invest', 'reinvest', 'other'], recvState: 'all' }
  showExportDialog.value = true
}

function applyExportFilter(list: Party[]): Party[] {
  const selectedTypes = exportFilter.value.types
  return list.filter(p => {
    // 类型匹配：party.types 或 party.type 任一在 selectedTypes 里就算匹配
    const partyTypes = p.types && p.types.length ? p.types : (p.type ? [p.type] : [])
    const typeMatch = partyTypes.some(t => selectedTypes.includes(t))
    if (!typeMatch) return false
    // 应收状态
    const outstanding = p.outstandingCents || 0
    if (exportFilter.value.recvState === 'has' && outstanding <= 0) return false
    if (exportFilter.value.recvState === 'none' && outstanding > 0) return false
    return true
  })
}

async function doExport() {
  if (exportPreviewCount.value === 0) {
    showToast('没有符合条件的单位')
    return
  }
  showExportDialog.value = false
  try {
    const filtered = applyExportFilter(parties.value)
    exportContactsExcel(filtered)
    showToast(`已导出 ${filtered.length} 个单位`)
  } catch (e: any) {
    showToast(e.message || '导出失败')
  }
}

function fenToYuan(cents: number | null | undefined): number {
  return Math.round((cents || 0) / 100 * 100) / 100
}

function exportContactsExcel(targetParties: Party[]): void {
  const wb = XLSX.utils.book_new()

  // --- Sheet 1: 往来单位 ---
  const partyHeaders = [
    '名称', '类型', '联系电话',
    '投资本金(元)', '收益率(%)', '年收益(元)',
    '流转亩数', '每亩流转费(元)', '总流转费(元)',
    '每亩管理费(元)', '总管理费(元)',
    '欠款(元)', '备注',
  ]
  const partyRows = targetParties.map(p => {
    const typesLabel = (p.types || []).map(t => partyTypeLabel[t] || t).join('/') || partyTypeLabel[p.type || ''] || ''
    return [
      p.name, typesLabel, p.contactPhone || '',
      fenToYuan(p.investAmountCents),
      p.returnRateBps ? (p.returnRateBps / 10000).toFixed(4) : '',
      fenToYuan(p.expectedReturnCents),
      p.landMu || p.areaMu || '',
      fenToYuan(p.landFeePerMuCents),
      fenToYuan(p.expectedLandFeeCents),
      fenToYuan(p.mgmtFeePerMuCents),
      fenToYuan(p.expectedMgmtFeeCents),
      fenToYuan(p.outstandingCents),
      p.note || '',
    ]
  })
  const wsParties = XLSX.utils.aoa_to_sheet([partyHeaders, ...partyRows])
  wsParties['!cols'] = [
    { wch: 25 }, { wch: 20 }, { wch: 15 },
    { wch: 14 }, { wch: 10 }, { wch: 14 },
    { wch: 10 }, { wch: 16 }, { wch: 14 },
    { wch: 16 }, { wch: 14 },
    { wch: 12 }, { wch: 20 },
  ]
  XLSX.utils.book_append_sheet(wb, wsParties, '往来单位')

  // --- Sheet 2: 应收汇总 ---
  // 按类型汇总每个单位的欠款 + 年度预计应收
  const recvHeaders = [
    '单位名称', '类型',
    '应收投资收益(元)', '应收土地流转费(元)', '应收流转管理费(元)', '应收再投资收益(元)',
    '已收合计(元)', '待收合计(元)',
  ]
  const recvRows = targetParties.map(p => {
    // 从 receivableItems + standards 汇总（当前页已加载的）
    const related = receivableItems.value.filter(r => r.partyId === p.id )
    const sumByKind = (kind: string) => fenToYuan(
      related.filter(r => r.recvKind === kind).reduce((s, r) => s + (r.outstandingCents || 0), 0)
    )
    const paidSum = fenToYuan(
      related.reduce((s, r) => s + (r.paidCents || 0), 0)
    )
    const outstandingTotal = fenToYuan(p.outstandingCents)
    return [
      p.name, (p.types || []).map(t => partyTypeLabel[t]).join('/') || partyTypeLabel[p.type || ''] || '',
      sumByKind('dividend'),
      sumByKind('rent'),
      sumByKind('service'),
      sumByKind('reinvest_dividend'),
      paidSum,
      outstandingTotal,
    ]
  })
  // 合计行
  const totalRow = [
    '合计', '',
    fenToYuan(targetParties.reduce((s, p) => {
      const related = receivableItems.value.filter(r => r.partyId === p.id  && r.recvKind === 'dividend')
      return s + related.reduce((ss, r) => ss + (r.outstandingCents || 0), 0)
    }, 0)),
    fenToYuan(targetParties.reduce((s, p) => {
      const related = receivableItems.value.filter(r => r.partyId === p.id  && r.recvKind === 'rent')
      return s + related.reduce((ss, r) => ss + (r.outstandingCents || 0), 0)
    }, 0)),
    fenToYuan(targetParties.reduce((s, p) => {
      const related = receivableItems.value.filter(r => r.partyId === p.id  && r.recvKind === 'service')
      return s + related.reduce((ss, r) => ss + (r.outstandingCents || 0), 0)
    }, 0)),
    fenToYuan(0),
    fenToYuan(targetParties.reduce((s, p) => s + (receivableItems.value.filter(r => r.partyId === p.id).reduce((ss, r) => ss + (r.paidCents || 0), 0)), 0)),
    fenToYuan(targetParties.reduce((s, p) => s + p.outstandingCents, 0)),
  ]
  const wsRecv = XLSX.utils.aoa_to_sheet([recvHeaders, ...recvRows, totalRow])
  wsRecv['!cols'] = [
    { wch: 25 }, { wch: 20 },
    { wch: 16 }, { wch: 18 }, { wch: 18 }, { wch: 18 },
    { wch: 14 }, { wch: 14 },
  ]
  XLSX.utils.book_append_sheet(wb, wsRecv, '应收汇总')

  const now = new Date()
  const fname = `集体台账-往来单位及应收-${now.getFullYear()}${String(now.getMonth()+1).padStart(2,'0')}${String(now.getDate()).padStart(2,'0')}.xlsx`
  XLSX.writeFile(wb, fname)
}

// ---- 往来单位 ----
const currentParty = ref<Party | null>(null)
const receivableItems = ref<Receivable[]>([])
const receiptsByRec = ref<Record<number, Receipt[]>>({})
const expandedReceipts = ref<Record<number, boolean>>({})

const showAddParty = ref(false)
const editParty = ref(false)

// 弹窗内合同上传（待保存 party 后再关联）
interface InlineContract { fileName: string; fileSize: number; mimeType: string; fileData: string; _status?: 'uploading' | 'ok' | 'error' }
const inlineContracts = ref<InlineContract[]>([])
const inlineUploading = ref(false)
const formTab = ref<'basic' | 'data'>('basic')

const partyForm = ref({
  id: 0,
  name: '',
  type: 'flow' as 'flow' | 'invest' | 'reinvest' | 'other',
  contactPhone: '',
  note: '',
  // 投资/再投资专属字段（invest / reinvest 类型用，二者内容一致）
  investAmountYuan: '',
  returnRatePercent: '',
  expectedReturnYuan: '',
  // 土地流转专属字段（flow 类型用）
  landMuYuan: '',
  landFeePerMuYuan: '',
  expectedLandFeeYuan: '',
  mgmtFeePerMuYuan: '',
  expectedMgmtFeeYuan: '',
})

const partyTypeLabel: Record<string, string> = { invest: '长期投资单位', reinvest: '再投资单位', flow: '土地流转企业', other: '其他单位' }
const partyTypeColor: Record<string, string> = { invest: '#1989fa', reinvest: '#ff6034', flow: '#07c160', other: '#969799' }
const ALL_PARTY_TYPES = [
  { value: 'invest' as const, label: '长期投资单位' },
  { value: 'reinvest' as const, label: '再投资单位' },
  { value: 'flow' as const, label: '土地流转企业' },
  { value: 'other' as const, label: '其他单位' },
]

// 自动计算辅助
function autoInvestReturn() {
  const amtYuan = parseFloat(partyForm.value.investAmountYuan || '0') || 0
  const rate = parseFloat(partyForm.value.returnRatePercent || '0') || 0
  return Math.round(amtYuan * rate)  // 年收益 = 投资金额(元) × 收益率(%)
}
function autoLandFee() {
  const mu = parseFloat(partyForm.value.landMuYuan || '0') || 0
  const perMu = parseFloat(partyForm.value.landFeePerMuYuan || '0') || 0
  return Math.round(mu * perMu)
}
function autoMgmtFee() {
  const mu = parseFloat(partyForm.value.landMuYuan || '0') || 0
  const perMu = parseFloat(partyForm.value.mgmtFeePerMuYuan || '0') || 0
  return Math.round(mu * perMu)
}

// 自动写入目标字段，用户手改后停止覆盖
let lastAutoReturn: number | null = null
let lastAutoLandFee: number | null = null
let lastAutoMgmt: number | null = null

watch(
  () => [partyForm.value.investAmountYuan, partyForm.value.returnRatePercent],
  () => {
    const auto = autoInvestReturn()
    const cur = parseFloat(partyForm.value.expectedReturnYuan || '0') || 0
    if (lastAutoReturn === null || cur === lastAutoReturn) {
      partyForm.value.expectedReturnYuan = auto > 0 ? String(auto) : ''
    }
    lastAutoReturn = auto
  }
)

watch(
  () => [partyForm.value.landMuYuan, partyForm.value.landFeePerMuYuan],
  () => {
    const auto = autoLandFee()
    const cur = parseFloat(partyForm.value.expectedLandFeeYuan || '0') || 0
    if (lastAutoLandFee === null || cur === lastAutoLandFee) {
      partyForm.value.expectedLandFeeYuan = auto > 0 ? String(auto) : ''
    }
    lastAutoLandFee = auto
  }
)

watch(
  () => [partyForm.value.landMuYuan, partyForm.value.mgmtFeePerMuYuan],
  () => {
    const auto = autoMgmtFee()
    const cur = parseFloat(partyForm.value.expectedMgmtFeeYuan || '0') || 0
    if (lastAutoMgmt === null || cur === lastAutoMgmt) {
      partyForm.value.expectedMgmtFeeYuan = auto > 0 ? String(auto) : ''
    }
    lastAutoMgmt = auto
  }
)

const showAddReceivable = ref(false)
const recvForm = ref({
  recvKind: 'rent' as RecvKind,
  title: '',
  amount: '',
  note: '',
})

// ---- 收款/抵销 ----
const showReceipt = ref(false)
const currentReceivable = ref<Receivable | null>(null)
const receiptForm = ref({
  method: 'cash' as 'cash' | 'offset',
  amount: '',
  date: todayStr(),
  categoryId: null as number | null,
  categoryName: '',
  txnId: null as number | null,
  txnLabel: '',
  note: '',
})
const savingReceipt = ref(false)

// 再投资比例设置（从 /settings 读取）
const reinvestRatioBps = ref(0)
let settingsLoaded = false
async function ensureSettings() {
  if (settingsLoaded) return
  try {
    const s = await api.get<ApiResponse<{ bankBalanceCents?: number; reinvestRatioBps?: number }>>('/settings')
    reinvestRatioBps.value = (s.data?.reinvestRatioBps as number) || 0
  } catch { /* 忽略 */ }
  settingsLoaded = true
}

// 再投资去向对话框（核销成功后按需弹出）
const showReinvestDialog = ref(false)
const reinvestForm = ref({
  receiptAmountCents: 0,
  reinvestAmountCents: 0,
  targetName: '',
  targetPartyId: null as number | null,
  notes: '',
})

const cats = ref<Category[]>([])
const incomeCatOptions = ref<{ name: string; value: number }[]>([])
const showIncomeCatPicker = ref(false)

const showTxnPicker = ref(false)
const offsetTxnOptions = ref<{ name: string; value: number }[]>([])
const catNameById = computed<Record<number, string>>(() => {
  const m: Record<number, string> = {}
  for (const l1 of cats.value) {
    if (l1.children) {
      for (const l2 of l1.children) m[l2.id] = `${l1.name} / ${l2.name}`
    }
  }
  return m
})

const cashCategoryResolved = computed(() => {
  if (receiptForm.value.method !== 'cash') return ''
  const id = receiptForm.value.categoryId
  if (id == null) return ''
  return '（入账科目：' + (catNameById.value[id] || `#${id}`) + '）'
})

onMounted(async () => {
  await Promise.all([loadParties(), loadCategories()])
})

// ---- 单位列表 ----
async function loadParties() {
  loading.value = true
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    parties.value = res.data
    void loadKindOwes()
  } catch {
    parties.value = []
  } finally {
    loading.value = false
  }
}

// ---- 分类欠款 banner：总计 / 流转费 / 投资收益 ----
const rentOweTotal = ref(0)
const dividendOweTotal = ref(0)
const activeTypeFilter = ref<'all' | 'flow' | 'invest'>('all')

const totalOwe = computed(() => parties.value.reduce((s, p) => s + (p.outstandingCents || 0), 0))

const displayParties = computed(() => {
  if (activeTypeFilter.value === 'flow') return parties.value.filter(p => p.type === 'flow')
  if (activeTypeFilter.value === 'invest') return parties.value.filter(p => p.type === 'invest')
  return parties.value
})

function toggleBanner(kind: 'all' | 'flow' | 'invest') {
  activeTypeFilter.value = activeTypeFilter.value === kind ? 'all' : kind
}

async function loadKindOwes() {
  try {
    const res = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', { status: 'open', pageSize: 1000 })
    let rentTotal = 0
    let dividendTotal = 0
    for (const it of res.data.items || []) {
      if (it.outstandingCents <= 0) continue
      if (it.recvKind === 'rent') rentTotal += it.outstandingCents
      else if (it.recvKind === 'dividend') dividendTotal += it.outstandingCents
    }
    rentOweTotal.value = rentTotal
    dividendOweTotal.value = dividendTotal
  } catch {
    rentOweTotal.value = 0
    dividendOweTotal.value = 0
  }
}

function openAddParty() {
  inlineContracts.value = []
  formTab.value = 'basic'
  editParty.value = false
  partyForm.value = {
    id: 0, name: '',
    type: 'flow',
    contactPhone: '', note: '',
    investAmountYuan: '', returnRatePercent: '', expectedReturnYuan: '',
    landMuYuan: '', landFeePerMuYuan: '', expectedLandFeeYuan: '',
    mgmtFeePerMuYuan: '', expectedMgmtFeeYuan: '',
  }
  showAddParty.value = true
}

function openAddPartyEdit() {
  inlineContracts.value = []
  formTab.value = 'basic'
  if (!currentParty.value) return
  const p = currentParty.value
  editParty.value = true
  const type = (p.type && ['flow','invest','reinvest','other'].includes(p.type)) ? p.type : 'flow'
  partyForm.value = {
    id: p.id,
    name: p.name,
    type,
    contactPhone: p.contactPhone || '',
    note: p.note || '',
    // 投资/再投资
    investAmountYuan: (p.investAmountCents / 100).toFixed(2),
    returnRatePercent: p.returnRateBps ? (p.returnRateBps / 10000).toFixed(4) : '',
    expectedReturnYuan: (p.expectedReturnCents / 100).toFixed(2),
    // 土地流转
    landMuYuan: p.landMu ? String(p.landMu) : (p.areaMu ? String(p.areaMu) : ''),
    landFeePerMuYuan: (p.landFeePerMuCents / 100).toFixed(2),
    expectedLandFeeYuan: (p.expectedLandFeeCents / 100).toFixed(2),
    mgmtFeePerMuYuan: (p.mgmtFeePerMuCents / 100).toFixed(2),
    expectedMgmtFeeYuan: (p.expectedMgmtFeeCents / 100).toFixed(2),
  }
  showAddParty.value = true
}

// 保存标准金额（无输入不覆盖旧值）
async function syncPartyStandard(partyId: number, kind: 'rent' | 'dividend', yuan: string) {
  const amount = Math.round(parseFloat(yuan || '0') * 100)
  if (amount <= 0) return
  await api.post('/recv-standards', { partyId, recvKind: kind, amountCents: amount })
}

async function saveParty() {
  if (!partyForm.value.name) {
    showToast('请填写单位名称')
    return
  }
  const f = partyForm.value
  // yuan → cents helper
  const toCents = (yuan: string) => Math.round(parseFloat(yuan || '0') * 100)
  const toYuan = (cents: string | number) => (typeof cents === 'number' ? cents : parseFloat(cents || '0') * 100)
  // 自动计算 expected_* 字段（用户没改的话用自动值）
  const autoER = autoInvestReturn()
  const autoLF = autoLandFee()
  const autoMF = autoMgmtFee()
  // 如果用户手动改了 expected 字段，优先用用户值；否则用自动值
  const expectedReturnInput = parseFloat(f.expectedReturnYuan || '0')
  const expectedLandFeeInput = parseFloat(f.expectedLandFeeYuan || '0')
  const expectedMgmtFeeInput = parseFloat(f.expectedMgmtFeeYuan || '0')

  const payload: Record<string, unknown> = {
    name: f.name,
    type: f.type,
    types: [f.type], // 后端兼容字段
    contactPhone: f.contactPhone.trim(),
    note: f.note,
    // invest 类型字段
    investAmountCents: toCents(f.investAmountYuan),
    returnRateBps: Math.round(parseFloat(f.returnRatePercent || '0') * 10000),
    expectedReturnCents: toCents(expectedReturnInput > 0 ? f.expectedReturnYuan : String(autoER)),
    // flow 字段
    landMu: parseFloat(f.landMuYuan || '0') || 0,
    areaMu: parseFloat(f.landMuYuan || '0') || 0, // 兼容旧字段
    landFeePerMuCents: toCents(f.landFeePerMuYuan),
    expectedLandFeeCents: toCents(expectedLandFeeInput > 0 ? f.expectedLandFeeYuan : String(autoLF)),
    mgmtFeePerMuCents: toCents(f.mgmtFeePerMuYuan),
    expectedMgmtFeeCents: toCents(expectedMgmtFeeInput > 0 ? f.expectedMgmtFeeYuan : String(autoMF)),
  }
  try {
    let id = f.id
    if (editParty.value) {
      await api.put(`/parties/${id}`, payload)
      showToast('更新成功')
    } else {
      const res = await api.post<ApiResponse<Party>>('/parties', payload)
      id = res.data?.id ?? 0
      showToast('创建成功')
    }
    // 保存弹窗内收集的合同
    if (inlineContracts.value.length > 0 && id > 0) {
      await uploadInlineContracts(id)
      inlineContracts.value = []
    }
    await loadParties()
    if (currentParty.value) {
      const updated = parties.value.find(p => p.id === currentParty.value!.id)
      if (updated) currentParty.value = updated
    }
  } catch (e: any) {
    if (e?.status === 409 && e?.response?.code === 'DUPLICATE_NAME') {
      const dupes = e.response.dupes || []
      const names = dupes.map((d: any) => d.name).join('、')
      showDialog({ title: '重名提示', message: `存在重名单位：${names}\n\n请修改单位名称后重试。` })
    } else {
      showDialog({ title: '保存失败', message: e.message || '保存失败，请重试' })
    }
  }
}

// ---- 详情（弹窗） ----
const showPartyDetail = ref(false)
const contracts = ref<Contract[]>([])
const contractUploading = ref(false)
const allocations = ref<ReinvestAllocation[]>([])
const showAllocForm = ref(false)
const allocForm = ref({ targetName: '', amount: '', notes: '' })
async function loadAllocations(partyId: number) {
  try {
    allocations.value = (await api.get<ApiResponse<ReinvestAllocation[]>>(`/parties/${partyId}/allocations`)).data || []
  } catch { allocations.value = [] }
}
async function saveAlloc() {
  if (!currentParty.value) return
  const amountCents = Math.round(parseFloat(allocForm.value.amount || '0') * 100)
  if (!allocForm.value.targetName.trim()) { showToast('请填写去向单位'); return }
  if (amountCents <= 0) { showToast('金额必须大于 0'); return }
  await api.post(`/parties/${currentParty.value.id}/allocations`, {
    targetName: allocForm.value.targetName.trim(),
    amountCents,
    notes: allocForm.value.notes || null,
  })
  showAllocForm.value = false
  allocForm.value = { targetName: '', amount: '', notes: '' }
  await loadAllocations(currentParty.value.id)
  showToast('已保存')
}
async function deleteAlloc(a: ReinvestAllocation) {
  try { await showDialog({ title: '确认删除', message: `删除对"${a.targetName}"的再投资去向？`, showCancelButton: true }) } catch { return }
  await api.del(`/allocations/${a.id}`)
  if (currentParty.value) await loadAllocations(currentParty.value.id)
}

async function saveReinvest() {
  if (!currentParty.value) return
  if (!reinvestForm.value.targetName.trim()) { showToast('请填写再投资去向名称'); return }
  if (reinvestForm.value.reinvestAmountCents <= 0) { showToast('再投资金额必须大于 0'); return }
  await api.post(`/parties/${currentParty.value.id}/allocations`, {
    targetName: reinvestForm.value.targetName.trim(),
    targetPartyId: reinvestForm.value.targetPartyId,
    amountCents: reinvestForm.value.reinvestAmountCents,
    notes: reinvestForm.value.notes || null,
  })
  showToast('再投资去向已登记')
  showReinvestDialog.value = false
  if (currentParty.value) await loadAllocations(currentParty.value.id)
}

function onPartyDetailClosed() {
  currentParty.value = null
  receivableItems.value = []
  contracts.value = []
  allocations.value = []
  showAllocForm.value = false
}

async function loadContracts(partyId: number) {
  try {
    contracts.value = (await api.get<ApiResponse<Contract[]>>('/contracts', { partyId })).data || []
  } catch {
    contracts.value = []
  }
}

// 智能拆分文件名：去掉 URL 风格的 query 串，提取真正的扩展名
function splitCleanFileName(raw: string): { cleanName: string; ext: string } {
  if (!raw) return { cleanName: '未命名', ext: '' }
  let name = raw
  // 去掉 URL query (第一个 ? 之后的)
  const qIdx = name.indexOf('?')
  if (qIdx > 0) name = name.slice(0, qIdx)
  // 去掉 URL path 里的 / (取最后一段)
  const slashIdx = Math.max(name.lastIndexOf('/'), name.lastIndexOf('\\'))
  if (slashIdx >= 0) name = name.slice(slashIdx + 1)
  // 拆扩展名
  const dotIdx = name.lastIndexOf('.')
  let base = name
  let ext = ''
  if (dotIdx > 0 && dotIdx < name.length - 1) {
    ext = name.slice(dotIdx)
    base = name.slice(0, dotIdx)
  }
  // 清洗 base：去掉明显的 hash/query 风格串
  // 如 "u=4217215850,4193273696&fm=253&fmt=auto&app=138&f=JPEG" → "image"
  if (/[=&%]/.test(base) || /^\w+=\d{4,}/.test(base)) {
    const map: Record<string, string> = {
      'image': '上传图片',
      'jpeg': '上传图片',
      'jpg': '上传图片',
      'png': '上传图片',
      'pdf': '上传文档',
      'doc': '上传文档',
      'docx': '上传文档',
      'xls': '上传表格',
      'xlsx': '上传表格',
    }
    const lowerExt = ext.replace('.', '').toLowerCase()
    base = map[lowerExt] || `上传文件-${Date.now().toString().slice(-6)}`
  }
  // 超长截断
  if (base.length > 50) base = base.slice(0, 50) + '…'
  return { cleanName: base || '未命名', ext }
}

// 合同列表显示名称：优先 contractTitle，但也要清洗脏的 contractTitle
function displayContractName(c: Contract): string {
  const rawTitle = c.contractTitle?.trim()
  if (rawTitle && !looksLikeDirtyFileName(rawTitle)) return rawTitle
  return splitCleanFileName(c.fileName).cleanName
}

// 合同列表显示的真实文件名（带扩展名）
function displayContractFileName(c: Contract): string {
  const { cleanName, ext } = splitCleanFileName(c.fileName)
  return cleanName + ext
}

// 检测是否为脏文件名（URL query / hash 风格）
function looksLikeDirtyFileName(s: string): boolean {
  if (!s) return false
  return /[=&%]/.test(s) || /^\w+=\d{4,}/.test(s) || /[?#]/.test(s) || s.length > 80
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(2) + ' MB'
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

async function onContractUpload(file: File) {
  if (!currentParty.value) return
  if (file.size > 5 * 1024 * 1024) {
    showToast('文件不能超过 5MB')
    return
  }
  contractUploading.value = true
  try {
    const dataUrl = await readFileAsDataURL(file)
    const { cleanName, ext } = splitCleanFileName(file.name)
    await api.post('/contracts', {
      partyId: currentParty.value.id,
      fileName: cleanName + ext,
      fileSize: file.size,
      mimeType: file.type || 'application/octet-stream',
      contractTitle: cleanName,
      contractDate: null,
      expiresAt: null,
      fileData: dataUrl,
    })
    showToast('上传成功')
    await loadContracts(currentParty.value.id)
  } catch (e: any) {
    showToast(e.message || '上传失败')
  } finally {
    contractUploading.value = false
  }
}

// 弹窗内合同上传：先收集在 inlineContracts 数组，保存 party 后再真正 POST
async function handleInlineUpload(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > 5 * 1024 * 1024) { showToast('文件不能超过 5MB'); return }
  inlineUploading.value = true
  try {
    const dataUrl = await readFileAsDataURL(file)
    const { cleanName, ext } = splitCleanFileName(file.name)
    inlineContracts.value.push({
      fileName: cleanName + ext,
      fileSize: file.size,
      mimeType: file.type || 'application/octet-stream',
      fileData: dataUrl,
      _status: 'ok',
    })
    showToast('已选择：' + cleanName + ext)
  } catch {
    showToast('读取文件失败')
  } finally {
    inlineUploading.value = false
  }
}

async function uploadInlineContracts(partyId: number) {
  for (const c of inlineContracts.value) {
    await api.post('/contracts', {
      partyId, fileName: c.fileName, fileSize: c.fileSize,
      mimeType: c.mimeType, contractTitle: splitCleanFileName(c.fileName).cleanName,
      contractDate: null, expiresAt: null, fileData: c.fileData,
    })
  }
}

async function previewContract(c: Contract) {
  try {
    const full = (await api.get<ApiResponse<Contract>>(`/contracts/${c.id}`)).data
    if (!full?.fileData) { showToast('文件内容为空'); return }
    // 解析 data:mime;base64,xxx → Blob → Blob URL
    const comma = full.fileData.indexOf(',')
    if (comma < 0) { showToast('文件格式错误'); return }
    const meta = full.fileData.slice(0, comma)
    const base64 = full.fileData.slice(comma + 1)
    const mimeMatch = meta.match(/data:([^;]+)/)
    const mime = mimeMatch ? mimeMatch[1] : (full.mimeType || 'application/octet-stream')
    const bin = atob(base64)
    const bytes = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
    const blob = new Blob([bytes], { type: mime })
    const blobUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = blobUrl
    a.target = '_blank'
    a.rel = 'noopener'
    a.download = full.fileName
    document.body.appendChild(a)
    a.click()
    setTimeout(() => {
      a.remove()
      URL.revokeObjectURL(blobUrl)
    }, 30000)
  } catch (e: any) {
    showToast(e.message || '预览失败')
  }
}

async function deleteContract(c: Contract) {
  try {
    await showDialog({ title: '确认删除', message: `确定删除"${c.fileName}"吗？`, showCancelButton: true })
  } catch { return }
  try {
    await api.del(`/contracts/${c.id}`)
    showToast('已删除')
    if (currentParty.value) await loadContracts(currentParty.value.id)
  } catch (e: any) {
    showToast(e.message || '删除失败')
  }
}

function contractIcon(fileName: string, mimeType?: string | null): string {
  const mime = mimeType || ''
  if (mime.startsWith('image/')) return 'photo-o'
  if (mime === 'application/pdf') return 'description'
  if (/\.(doc|docx)$/i.test(fileName)) return 'description'
  if (/\.(xls|xlsx)$/i.test(fileName)) return 'orders-o'
  if (/\.(zip|rar|7z)$/i.test(fileName)) return 'passed'
  return 'records'
}

async function openDetail(p: Party) {
  currentParty.value = p
  showPartyDetail.value = true
  detailLoading.value = true
  receivableItems.value = []
  receiptsByRec.value = {}
  expandedReceipts.value = {}
  void loadStandards()
  void loadContracts(p.id)
  void loadAllocations(p.id)
  try {
    const res = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', { partyId: p.id, pageSize: 200 })
    receivableItems.value = res.data.items || []
    for (const r of receivableItems.value) {
      if (r.status === 'open' || r.paidCents > 0) {
        await loadReceipts(r)
      }
    }
    // 刷新对象欠款合计（应收单核销会改变欠款）
    await loadParties()
    const fresh = parties.value.find(x => x.id === p.id)
    if (fresh) currentParty.value = fresh
  } catch {
    receivableItems.value = []
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  showPartyDetail.value = false
  currentParty.value = null
}

async function loadReceipts(r: Receivable) {
  try {
    const res = await api.get<ApiResponse<ReceivableDetail>>(`/receivables/${r.id}`)
    receiptsByRec.value[r.id] = res.data.receipts || []
  } catch {
    receiptsByRec.value[r.id] = []
  }
}

function toggleReceipts(r: Receivable) {
  expandedReceipts.value[r.id] = !expandedReceipts.value[r.id]
}

// ---- 登记应收 ----
function openAddReceivable() {
  recvForm.value = { recvKind: 'rent', title: '', amount: '', note: '' }
  showAddReceivable.value = true
}

async function saveReceivable() {
  if (!currentParty.value) return
  const amountCents = parseFen(recvForm.value.amount)
  if (amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  if (!recvForm.value.title) {
    showToast('请填写事由')
    return
  }
  try {
    const payload: Record<string, unknown> = {
      partyId: currentParty.value.id,
      recvKind: recvForm.value.recvKind,
      title: recvForm.value.title,
      amountCents,
    }
    if (recvForm.value.note) payload.note = recvForm.value.note
    await api.post('/receivables', payload)
    showToast('登记成功')
    showAddReceivable.value = false
    await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '登记失败', message: e.message || '登记失败，请重试' })
  }
}

// ---- 收款 / 抵销 ----
function openReceipt(r: Receivable) {
  currentReceivable.value = r
  receiptForm.value = {
    method: 'cash',
    amount: String((r.outstandingCents / 100).toFixed(2)),
    date: todayStr(),
    categoryId: null,
    categoryName: '',
    txnId: null,
    txnLabel: '',
    note: '',
  }
  showReceipt.value = true
  void fetchOffsetOptions()
}

const canSubmitReceipt = computed(() => {
  const amount = parseFen(receiptForm.value.amount)
  if (amount <= 0) return false
  if (receiptForm.value.method === 'offset' && !receiptForm.value.txnId) return false
  if (receiptForm.value.method === 'cash' && !receiptForm.value.categoryId) return false
  return true
})

async function saveReceipt() {
  const amountCents = parseFen(receiptForm.value.amount)
  if (!currentReceivable.value || amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  savingReceipt.value = true
  try {
    const payload: Record<string, unknown> = {
      amountCents,
      receiptDate: receiptForm.value.date,
      method: receiptForm.value.method,
    }
    if (receiptForm.value.method === 'cash') {
      payload.categoryId = receiptForm.value.categoryId
    }
    if (receiptForm.value.method === 'offset') {
      payload.txnId = receiptForm.value.txnId
    }
    if (receiptForm.value.note) payload.note = receiptForm.value.note
    await api.post(`/receivables/${currentReceivable.value.id}/receipts`, payload)
    showToast('核销成功')
    showReceipt.value = false

    // 再投资钩子：如果 settings 配了 reinvestRatioBps，弹框提示登记再投资去向
    await ensureSettings()
    if (reinvestRatioBps.value > 0 && currentParty.value) {
      const reinvestCents = Math.round(amountCents * reinvestRatioBps.value / 10000)
      reinvestForm.value = {
        receiptAmountCents: amountCents,
        reinvestAmountCents: reinvestCents,
        targetName: '',
        targetPartyId: null,
        notes: `核销 ${formatFen(amountCents)} 按 ${(reinvestRatioBps.value / 100).toFixed(0)}% 比例自动登记`,
      }
      showReinvestDialog.value = true
    }

    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '核销失败', message: e.message || '核销失败，请重试' })
  } finally {
    savingReceipt.value = false
  }
}

// 预载发放支出流水选项（桌面下拉用；不主动弹层）
async function fetchOffsetOptions() {
  offsetTxnOptions.value = []
  try {
    const res = await api.get<ApiResponse<{ items: Transaction[] }>>('/transactions', { pageSize: 50 })
    const options: { name: string; value: number }[] = []
    for (const t of res.data.items || []) {
      if (t.direction === 'expense' && t.status === 'normal') {
        const cat = catNameById.value[t.categoryId] || `#${t.categoryId}`
        const note = t.note ? ` ${t.note}` : ''
        options.push({ name: `${t.txnDate} ${cat}${note} ${formatFen(t.amountCents)}`, value: t.id })
      }
    }
    offsetTxnOptions.value = options
  } catch {
    offsetTxnOptions.value = []
  }
}

// 抵销需要选择发放支出流水（移动端弹 action sheet）
async function loadOffsetTxns() {
  await fetchOffsetOptions()
  showTxnPicker.value = true
  if (offsetTxnOptions.value.length === 0) showToast('近期没有可抵销的支出流水（如分红发放）')
}

function onTxnSelect(action: { name: string; value: number }) {
  receiptForm.value.txnId = action.value
  receiptForm.value.txnLabel = action.name
  showTxnPicker.value = false
}

async function voidReceipt(rc: Receipt) {
  try {
    await showDialog({
      title: '确认作废',
      message: `作废这笔${rc.method === 'cash' ? '现金核销' : '抵销'}（${formatFen(rc.amountCents)}）？若为现金核销，对应的银行收入流水将同步作废。`,
      showCancelButton: true,
    })
  } catch {
    return
  }
  try {
    await api.put(`/receipts/${rc.id}`, { status: 'voided' })
    showToast('已作废')
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

// ---- 年度结转 / 标准 ----
const showAccrue = ref(false)
const accrueTab = ref('accrue')
const accrueYear = ref(String(new Date().getFullYear()))
const accrueTitle = ref('')
const savingAccrue = ref(false)
const accrueRows = ref<{ partyId: number | null; partyName: string; recvKind: RecvKind; amountYuan: string }[]>([
  { partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' },
])

// 年度结转预览（从单位年度标准带数据）
const previewItems = ref<AccruePreviewItem[]>([])
const previewLoading = ref(false)
const previewLoaded = ref(false)
const previewNewCount = computed(() => previewItems.value.filter(it => !it.exists).length)

const showAccrueUnitPicker = ref(false)
let accruePickFor = 'row' as 'row' | 'std'
let accruePickRow = 0

const accrueUnitActions = computed(() =>
  parties.value.map(p => ({ name: p.name, value: p.id })),
)

const standards = ref<AccrualStandard[]>([])
const stdForm = ref<{ partyId: number | null; partyName: string; amountYuan: string }>({ partyId: null, partyName: '', amountYuan: '' })
const stdSelectedType = computed(() => {
  const p = parties.value.find(x => x.id === stdForm.value.partyId)
  return p?.type || 'flow'
})
const stdAmountLabel = computed(() => (stdSelectedType.value === 'invest' ? '年度应得分红' : '年度流转费'))
const stdKindError = computed(() => {
  return stdForm.value.partyId != null && stdSelectedType.value === 'other'
})
const savingStd = ref(false)
const accruing = ref(false)

async function previewAccrue() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year || year < 2000 || year > 2100) {
    showToast('请填写正确年度')
    return
  }
  previewLoading.value = true
  try {
    const res = await api.get<ApiResponse<AccruePreview>>('/recv-standards/preview', { year })
    previewItems.value = res.data?.items || []
    previewLoaded.value = true
    if (previewItems.value.length === 0) showToast('没有可结转的标准数据')
  } catch (e: any) {
    showToast(e.message || '生成预览失败')
  } finally {
    previewLoading.value = false
  }
}

async function openAccrue() {
  accrueTab.value = 'accrue'
  accrueYear.value = String(new Date().getFullYear())
  accrueTitle.value = ''
  accrueRows.value = [{ partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' }]
  stdForm.value = { partyId: null, partyName: '', amountYuan: '' }
  previewItems.value = []
  previewLoaded.value = false
  showAccrue.value = true
  await loadStandards()
  await loadParties()
}

// 当前单位按类型的年度标准（flow→流转费 / invest→应得分红）
const currentPartyStd = computed(() => {
  const p = currentParty.value
  if (!p || p.type === 'other') return null
  const kind = p.type === 'invest' ? 'dividend' : 'rent'
  return standards.value.find(s => s.partyId === p.id && s.recvKind === kind) || null
})

// 年度标准就地修改（单位详情“去修改”直接弹窗）
const showStdEditDialog = ref(false)
const stdEditYuan = ref('')
const stdEditLabel = computed(() => (currentParty.value?.type === 'invest' ? '年度应得分红' : '年度流转费'))
const stdEditTitle = computed(() => (currentParty.value?.type === 'invest' ? '修改年度应得分红' : '修改年度流转费'))

function openStdForParty() {
  stdEditYuan.value = currentPartyStd.value ? (currentPartyStd.value.amountCents / 100).toFixed(2) : ''
  showStdEditDialog.value = true
}

async function saveStdFromDetail() {
  const amount = Math.round(parseFloat(stdEditYuan.value || '0') * 100)
  const p = currentParty.value
  if (!p || p.type === 'other') return
  if (amount <= 0) {
    showToast('请填写正确的金额')
    return
  }
  try {
    await api.post('/recv-standards', {
      partyId: p.id,
      recvKind: p.type === 'invest' ? 'dividend' : 'rent',
      amountCents: amount,
    })
    showStdEditDialog.value = false
    showToast('已保存')
    await loadStandards()
    if (currentParty.value) {
      const updated = parties.value.find(x => x.id === currentParty.value!.id)
      if (updated) currentParty.value = updated
    }
  } catch (e: any) {
    showToast(e.message || '保存失败')
  }
}

function addAccrueRow() {
  accrueRows.value.push({ partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' })
}

function openAccrueUnit(kind: 'row' | 'std', rowIndex: number) {
  accruePickFor = kind
  accruePickRow = rowIndex
  showAccrueUnitPicker.value = true
}

function onAccrueUnitSelect(action: { name: string; value: number }) {
  if (accruePickFor === 'row') {
    const row = accrueRows.value[accruePickRow]
    if (row) {
      row.partyId = action.value
      row.partyName = action.name
    }
  } else {
    stdForm.value.partyId = action.value
    stdForm.value.partyName = action.name
  }
  showAccrueUnitPicker.value = false
}

async function saveBatch() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year || year < 2000 || year > 2100) {
    showToast('请填写正确年度')
    return
  }
  if (!accrueTitle.value.trim()) {
    showToast('请填写事由')
    return
  }
  const items: { partyId: number; recvKind: RecvKind; amountCents: number }[] = []
  for (const row of accrueRows.value) {
    const amount = parseFen(row.amountYuan)
    if (!row.partyId || amount <= 0) {
      showToast('请补全各单位与金额')
      return
    }
    items.push({ partyId: row.partyId, recvKind: row.recvKind, amountCents: amount })
  }
  savingAccrue.value = true
  try {
    const res = await api.post<ApiResponse<AccrueResult>>('/receivables/batch', {
      recvYear: year,
      title: accrueTitle.value.trim(),
      items,
    })
    showToast(`新增 ${res.data.created} 条${res.data.skipped ? `，跳过 ${res.data.skipped} 条` : ''}`)
    showAccrue.value = false
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '计提失败', message: e.message || '批量计提失败' })
  } finally {
    savingAccrue.value = false
  }
}

async function loadStandards() {
  try {
    const res = await api.get<ApiResponse<AccrualStandard[]>>('/recv-standards')
    standards.value = res.data || []
  } catch {
    standards.value = []
  }
}

async function saveStandard() {
  const amount = parseFen(stdForm.value.amountYuan)
  if (!stdForm.value.partyId || amount <= 0) {
    showToast('请选择单位并填写金额')
    return
  }
  if (stdKindError.value) {
    showToast('其它单位不设年度标准，需要时手动登记应收')
    return
  }
  savingStd.value = true
  try {
    const kind = stdSelectedType.value === 'invest' ? 'dividend' : 'rent'
    await api.post('/recv-standards', {
      partyId: stdForm.value.partyId,
      recvKind: kind,
      amountCents: amount,
    })
    showToast('已保存')
    stdForm.value = { partyId: null, partyName: '', amountYuan: '' }
    await loadStandards()
  } catch (e: any) {
    showToast(e.message || '保存失败')
  } finally {
    savingStd.value = false
  }
}

async function toggleStandard(s: AccrualStandard, v: boolean) {
  try {
    await api.put(`/recv-standards/${s.id}`, { active: v })
    s.active = v
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

async function accrueNow() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year) {
    showToast('请填写年度')
    return
  }
  accruing.value = true
  try {
    const res = (await api.post<ApiResponse<AccrueResult>>('/recv-standards/accrue', { year })).data
    showToast(`结转完成：新增 ${res.created} 条${res.skipped ? `，跳过 ${res.skipped} 条` : ''}`)
    previewItems.value = []
    previewLoaded.value = false
    await loadParties()
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '结转失败')
  } finally {
    accruing.value = false
  }
}

// ---- 应收作废（仅未收款，作废后可按标准重结） ----
async function voidReceivable(r: Receivable) {
  try {
    await showDialog({
      title: '确认作废',
      message: `作废应收「${r.title}」（${formatFen(r.amountCents)}）？仅未收款的应收可作废，作废后可在年度标准里重新结转。`,
      showCancelButton: true,
    })
  } catch {
    return // 用户取消
  }
  try {
    await api.put(`/receivables/${r.id}/void`)
    showToast('已作废')
    await loadParties()
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '作废失败')
  }
}

// ---- 科目与选择器 ----
async function loadCategories() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    cats.value = res.data
    const options: { name: string; value: number }[] = []
    for (const l1 of res.data) {
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status === 'active' && l2.kind === 'equity') {
            options.push({ name: `${l1.name} / ${l2.name}`, value: l2.id })
          }
        }
      }
    }
    incomeCatOptions.value = options
  } catch {
    cats.value = []
  }
}

function openIncomeCatPicker() {
  showIncomeCatPicker.value = true
}

function onIncomeCatSelect(action: { name: string; value: number }) {
  receiptForm.value.categoryId = action.value
  receiptForm.value.categoryName = action.name
  showIncomeCatPicker.value = false
}

function parseFen(yuan: string): number {
  const cleaned = yuan.replace(/[^0-9.]/g, '')
  const num = parseFloat(cleaned)
  if (isNaN(num)) return 0
  return Math.round(num * 100)
}
</script>

<style scoped>
.contacts-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: var(--paper);
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  margin: 0;
}

.detail-title {
  font-size: 16px;
  font-weight: 500;
  flex: 1;
  text-align: center;
}

.back-icon {
  font-size: 18px;
  color: var(--ink-soft);
  cursor: pointer;
}

.header-actions {
  display: flex;
  gap: 4px;
}

.kind-tabs {
  margin-bottom: 8px;
}

.loading-state {
  padding: 16px;
  background: #fff;
  border-radius: 12px;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  background: #fff;
  border-radius: 12px;
  color: var(--ink-muted);
}

.empty-state p {
  margin-bottom: 16px;
}

.party-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.party-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  cursor: pointer;
}

.party-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.party-name {
  font-size: 15px;
  font-weight: 500;
}

.owed-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--expense);
  font-variant-numeric: tabular-nums;
}

.owed-value.zero {
  color: var(--indigo);
}

.owed-label {
  font-size: 11px;
  color: var(--ink-muted);
  text-align: right;
}

.party-summary {
  display: flex;
  align-items: center;
  gap: 24px;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 12px;
}

.summary-value {
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.summary-value.zero {
  color: var(--indigo);
}

.summary-label {
  font-size: 11px;
  color: var(--ink-muted);
}

.recv-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.recv-card {
  background: #fff;
  border-radius: 12px;
  padding: 12px 16px;
}

.recv-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
}

.recv-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.recv-title {
  font-size: 14px;
  font-weight: 500;
}

.recv-amounts {
  display: flex;
  gap: 24px;
  padding: 8px 0;
}

.amount-cell {
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.amount-cell.strong {
  font-weight: 600;
  color: var(--expense);
}

.amount-label {
  font-size: 11px;
  color: var(--ink-muted);
}

.recv-actions {
  display: flex;
  gap: 4px;
  justify-content: flex-end;
}

.receipt-list {
  border-top: 1px solid var(--line-soft);
  margin-top: 8px;
  padding-top: 4px;
}

.receipt-empty {
  text-align: center;
  color: var(--ink-muted);
  font-size: 12px;
  padding: 8px 0;
}

.receipt-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}

.receipt-row.voided {
  opacity: 0.6;
}

.receipt-method {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid var(--indigo);
  color: var(--indigo);
}

.receipt-method.offset {
  border-color: var(--indigo);
  color: var(--indigo);
}

.receipt-amount {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.receipt-date {
  font-size: 12px;
  color: var(--ink-muted);
}

.l2-chip {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: var(--ink-muted);
}

.l2-chip.household { border-color: var(--indigo); color: var(--indigo); }
.l2-chip.unit { border-color: var(--indigo); color: var(--indigo); }
.l2-chip.rent { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.l2-chip.dividend { border-color: var(--indigo); color: var(--indigo); background: var(--indigo-light); }
.l2-chip.other { border-color: var(--ink-muted); color: var(--ink-soft); }
.l2-chip.open { border-color: #e88a3a; color: #a8601a; background: #fef3e8; }
.l2-chip.closed { border-color: var(--indigo); color: var(--indigo); background: var(--indigo-light); }
.l2-chip.stopped { background: #fcebeb; border-color: var(--expense); color: var(--expense); }

.receipt-popup {
  padding: 16px 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.popup-title {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  padding: 0 16px 12px;
}

.static-text {
  font-size: 13px;
  color: var(--ink);
}

.dialog-tip {
  font-size: 12px;
  color: var(--ink-muted);
  padding: 0 16px 8px;
  line-height: 1.5;
}

.receipt-save {
  margin: 8px 16px 0;
}

.accrue-popup {
  padding-bottom: 24px;
}

.accrue-body {
  padding: 8px 16px;
}

.accrue-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.accrue-row {
  background: var(--paper);
  border-radius: 8px;
  padding: 4px 8px;
}

.accrue-row-foot {
  display: flex;
  align-items: center;
  gap: 8px;
}

.accrue-add {
  padding: 8px 0;
}

.accrue-save {
  margin-top: 8px;
}

.accrue-empty {
  text-align: center;
  color: var(--ink-muted);
  font-size: 13px;
  padding: 16px 0;
}

.std-form {
  padding-bottom: 4px;
}

.std-list {
  margin-top: 8px;
  border-top: 1px solid var(--line-soft);
}

.std-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid var(--line-soft);
}

.std-name {
  font-size: 14px;
}

.std-amount {
  font-size: 13px;
  color: var(--ink-soft);
  font-variant-numeric: tabular-nums;
}

.summary-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 4px 0;
}

.summary-meta .meta-line {
  font-size: 12px;
  color: var(--ink-muted);
}

.l2-chip.flow { border-color: var(--indigo); color: var(--indigo); background: var(--indigo-light); }
.l2-chip.invest { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.l2-chip.other { border-color: var(--ink-muted); color: var(--ink-soft); }
.l2-chip.invest-chip { border-color: var(--indigo); color: var(--indigo); background: var(--indigo-light); }

.std-edit-link {
  color: var(--indigo);
  margin-left: 6px;
  cursor: pointer;
}

.preview-list {
  margin: 4px 0;
}

.preview-state {
  font-size: 12px;
  color: var(--indigo);
  white-space: nowrap;
}

.preview-state.dup {
  color: var(--ink-muted);
}

.party-detail-popup {
  padding-bottom: 24px;
}

.party-detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 16px 16px 0;
}

.party-detail-head .detail-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pd-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.party-type-select {
  flex: 1;
  width: 100%;
  height: 40px;
  border: 1px solid var(--line);
  border-radius: 8px;
  font-size: 15px;
  padding: 0 10px;
  background: #fff;
  color: var(--ink);
}

.owe-banners {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}

.owe-banner {
  flex: 1 1 0%;
  border-radius: 12px;
  padding: 12px 14px;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.owe-banner.total {
  background: var(--indigo);
  color: #fff;
}

.owe-banner.rent {
  background: var(--indigo-light);
  color: var(--indigo);
}

.owe-banner.invest {
  background: #fdf3e3;
  color: #7a4f0f;
}

.owe-banner.active {
  box-shadow: 0 0 0 2px var(--indigo) inset;
}

.owe-banner .banner-label {
  font-size: 12px;
  opacity: 0.85;
}

.owe-banner .banner-value {
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  margin-top: 2px;
}

/* ---- 详情弹窗站点风格美化 ---- */
.party-detail-popup {
  background: var(--paper);
}

.party-detail-head {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 12px;
  padding: 20px 16px 16px;
  background: linear-gradient(135deg, var(--indigo) 0%, #12569b 100%);
  color: #fff;
}

.pd-title {
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 0;
  min-width: 0;
}

.party-detail-head .detail-title {
  margin: 0;
  padding: 0;
  font-size: 18px;
  font-weight: 600;
  color: #fff;
  flex: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pd-type-text {
  font-size: 12px;
  font-weight: 400;
  color: rgba(255, 255, 255, 0.85);
  white-space: nowrap;
  margin-left: 2px;
  flex: none;
}

.pd-type-text::before {
  content: '';
}

/* 基本情况标题栏的编辑链接（白底场景） */
.section-title .pd-edit-link {
  margin-left: auto;
  font-size: 13px;
  font-weight: 400;
  color: #1989fa;
  text-decoration: none;
  padding: 0 6px;
}

.section-title .pd-edit-link:active {
  opacity: .7;
}

.detail-block {
  margin: 10px 12px 0;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--ink);
  padding: 2px 0 8px;
}

.section-title::before {
  content: '';
  width: 4px;
  height: 14px;
  border-radius: 2px;
  background: var(--indigo);
}

.party-detail-popup .party-summary {
  margin: 0;
  flex-wrap: wrap;
  gap: 10px 24px;
  box-shadow: 0 1px 4px rgba(44, 44, 42, 0.06);
}

.party-detail-popup .party-summary .summary-item {
  flex: 1 1 30%;
  min-width: 120px;
}

.party-detail-popup .party-summary .summary-meta {
  width: 100%;
  border-top: 1px dashed #e3e2dd;
  padding-top: 10px;
  margin-top: 4px;
}

.party-detail-popup .recv-list {
  padding: 0 0 16px;
  gap: 10px;
}

.party-detail-popup .recv-card {
  box-shadow: 0 1px 4px rgba(44, 44, 42, 0.06);
}

.party-detail-popup .recv-amounts {
  justify-content: space-between;
  gap: 8px;
  background: var(--paper);
  border-radius: 8px;
  padding: 8px 10px;
  margin-top: 8px;
}

.party-detail-popup .amount-cell {
  flex-direction: column;
  gap: 0;
  text-align: center;
}

.party-detail-popup .recv-actions {
  margin-top: 4px;
}

.auto-hint {
  color: #969799;
  font-size: 12px;
}
.auto-val {
  color: #07c160;
  font-size: 12px;
  font-weight: 500;
}
.party-edit-dialog .van-checkbox-group {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.section-subtitle {
  font-size: 12px;
  color: var(--ink-muted, #969799);
  font-weight: normal;
}

/* 合同附件 */
.contract-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.contract-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: var(--bg-soft, #f7f8fa);
  border-radius: 8px;
}
.contract-icon {
  font-size: 24px;
  color: var(--jade, #07c160);
  flex-shrink: 0;
}
.contract-info {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}
.contract-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--ink, #323233);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.contract-meta {
  font-size: 12px;
  color: var(--ink-muted, #969799);
  margin-top: 2px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.contract-upload {
  margin-top: 12px;
}
.hidden-file-input {
  display: none;
}
.upload-trigger {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 12px;
  border: 1px dashed var(--jade, #07c160);
  border-radius: 8px;
  color: var(--jade, #07c160);
  cursor: pointer;
  font-size: 14px;
  transition: background .15s;
}
.upload-trigger:active {
  background: rgba(7, 193, 96, 0.08);
}
.upload-hint {
  font-size: 12px;
  color: var(--ink-muted, #969799);
  text-align: center;
  margin-top: 6px;
}

/* 再投资去向 */
.alloc-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.alloc-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: var(--bg-soft, #f7f8fa);
  border-radius: 8px;
}
.alloc-main { flex: 1; min-width: 0; }
.alloc-target {
  font-size: 14px;
  font-weight: 500;
  color: var(--ink, #323233);
}
.alloc-meta {
  font-size: 12px;
  color: var(--ink-muted, #969799);
  margin-top: 2px;
  display: flex;
  gap: 10px;
}
.alloc-notes {
  font-size: 12px;
  color: var(--ink-muted, #969799);
  margin-top: 4px;
}
.alloc-del {
  font-size: 20px;
  color: var(--ink-muted, #969799);
  cursor: pointer;
  flex-shrink: 0;
}
.alloc-del:active { color: #ee0a24; }

/* === 新增/编辑单位：宽弹窗 + 两栏 grid === */
.party-edit-dialog {
  width: 640px !important;
  max-width: 95vw;
}
.party-edit-dialog .van-dialog__body {
  max-height: 75vh;
  overflow-y: auto;
  padding: 0 0 12px;
}
.party-edit-dialog :deep(.van-field) {
  flex-wrap: nowrap !important;
}
.party-edit-dialog :deep(.van-field__label) {
  width: 150px;
  flex: none;
  min-width: 150px;
  white-space: nowrap;
}
.party-edit-dialog :deep(.van-field__label > span) {
  white-space: nowrap;
}
.party-edit-dialog :deep(.van-field__control) {
  flex: 1 1 0;
  min-width: 0;
}
.party-edit-dialog :deep(.van-field__control input) {
  white-space: nowrap;
}
.party-edit-dialog :deep(.van-cell:last-child)::after {
  display: block !important;
}
.party-form-tabs {
  background: #fff;
}
.party-form-tabs .van-tab {
  font-size: 14px;
}
.party-form-tabs .van-tab--active {
  color: #1989fa;
}
.party-form-tabs__content {
  padding: 14px 16px 4px;
}
.party-form-tabs .van-cell-group.inset {
  margin: 0 0 10px;
  border-radius: 8px;
}
@media (max-width: 600px) {
  .party-edit-dialog { width: 96vw !important; }
}

/* 弹窗内 inline 合同上传 */
.inline-upload {
  padding: 8px 0;
}
.upload-trigger.inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border: 1px dashed #dcdee0;
  border-radius: 6px;
  color: #1989fa;
  font-size: 13px;
  cursor: pointer;
  transition: border-color .15s;
}
.upload-trigger.inline:hover { border-color: #1989fa; }
.inline-contract-list {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.inline-contract-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: #f7f8fa;
  border-radius: 6px;
  font-size: 13px;
}
.inline-ctitle {
  flex: 1;
  color: #323233;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.inline-cstate { color: #969799; font-size: 12px; }
.inline-cstate.ok { color: #07c160; }
.inline-cremove {
  color: #969799;
  cursor: pointer;
  padding: 2px;
}
.inline-cremove:hover { color: #ee0a24; }
</style>
















