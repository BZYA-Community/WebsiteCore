<template>
    <div>
        <main-nav :title="t('adminSettings.title')" />

        <n-card :title="t('adminSettings.title')" size="small" class="setting-card">
            <n-spin :show="loading">
                <n-space vertical size="large" class="settings-layout">
                    <n-alert
                        v-if="hasPendingRestart"
                        type="warning"
                        class="setting-alert"
                    >
                        {{ t('adminSettings.alert.pendingRestart') }}
                    </n-alert>

                    <div v-if="activeDomains.length === 0" class="empty-wrap">
                        <n-empty size="large" :description="t('adminSettings.empty')" />
                    </div>

                    <div
                        v-for="group in activeDomains"
                        :key="group.key"
                        class="domain-block"
                    >
                        <div class="domain-header">
                            <div>
                                <div class="domain-title">{{ group.label }}</div>
                                <div class="domain-subtitle">
                                    {{ t('adminSettings.itemCount', { count: group.itemCount }) }}
                                </div>
                            </div>
                            <n-tag round size="small">{{ group.key }}</n-tag>
                        </div>

                        <n-card
                            v-for="section in group.sections"
                            :key="`${group.key}-${section.key}`"
                            size="small"
                            class="section-card"
                            embedded
                            :bordered="false"
                        >
                            <template #header>
                                <div class="section-header">
                                    <div class="section-title">{{ section.label }}</div>
                                    <div class="section-subtitle">
                                        {{ t('adminSettings.sectionCount', { count: section.items.length }) }}
                                    </div>
                                </div>
                            </template>

                            <div
                                v-for="entry in section.items"
                                :key="entry.schema.key"
                                class="setting-item"
                            >
                                <div class="setting-item-top">
                                    <div>
                                        <div class="setting-item-title">
                                            {{ entry.schema.label }}
                                        </div>
                                        <div class="setting-item-key">
                                            {{ entry.schema.key }}
                                        </div>
                                    </div>
                                    <n-space size="small">
                                        <n-tag
                                            round
                                            size="small"
                                            :type="
                                                applyModeTagType(entry.schema.apply_mode)
                                            "
                                        >
                                            {{ applyModeLabel(entry.schema.apply_mode) }}
                                        </n-tag>
                                        <n-tag
                                            round
                                            size="small"
                                            :type="sourceTagType(entry.value?.source)"
                                        >
                                            {{ sourceLabel(entry.value?.source) }}
                                        </n-tag>
                                        <n-tag
                                            v-if="entry.schema.secret"
                                            round
                                            size="small"
                                        >
                                            {{ t('adminSettings.tag.secret') }}
                                        </n-tag>
                                        <n-tag
                                            v-if="entry.value?.pending_restart"
                                            round
                                            size="small"
                                            type="warning"
                                        >
                                            {{ t('adminSettings.tag.pendingRestart') }}
                                        </n-tag>
                                        <n-tag
                                            v-if="
                                                entry.schema.readonly ||
                                                entry.schema.apply_mode ===
                                                    'bootstrap_only'
                                            "
                                            round
                                            size="small"
                                            type="default"
                                        >
                                            {{ t('adminSettings.tag.readonly') }}
                                        </n-tag>
                                    </n-space>
                                </div>

                                <div class="setting-item-description">
                                    {{ entry.schema.description }}
                                </div>

                                <div class="setting-meta">
                                    <span class="meta-item">
                                        {{ t('adminSettings.label.currentStatus') }}{{ currentStatusText(entry) }}
                                    </span>
                                    <span class="meta-item">
                                        {{ t('adminSettings.label.bootstrapBaseline') }}{{ bootstrapStatusText(entry.schema) }}
                                    </span>
                                    <span
                                        v-if="showEffectiveValue(entry)"
                                        class="meta-item"
                                    >
                                        {{ t('adminSettings.label.currentEffective') }}{{
                                            formatValue(
                                                entry.value?.effective_value,
                                                entry.schema
                                            )
                                        }}
                                    </span>
                                </div>

                                <div class="setting-editor">
                                    <template v-if="entry.schema.secret">
                                        <n-input
                                            v-model:value="
                                                secretDraftValues[entry.schema.key]
                                            "
                                            type="password"
                                            show-password-on="click"
                                            :disabled="!isEditable(entry.schema)"
                                            :placeholder="t('adminSettings.placeholder.secretInput')"
                                        />
                                    </template>
                                    <template v-else-if="entry.schema.type === 'bool'">
                                        <n-switch
                                            v-model:value="draftValues[entry.schema.key]"
                                            :disabled="!isEditable(entry.schema)"
                                        />
                                    </template>
                                    <template v-else-if="hasOptions(entry.schema)">
                                        <n-select
                                            v-model:value="draftValues[entry.schema.key]"
                                            :options="settingOptions(entry.schema)"
                                            :disabled="!isEditable(entry.schema)"
                                        />
                                    </template>
                                    <template
                                        v-else-if="
                                            entry.schema.type === 'int' ||
                                            entry.schema.type === 'float'
                                        "
                                    >
                                        <n-input-number
                                            v-model:value="draftValues[entry.schema.key]"
                                            class="number-input"
                                            :precision="
                                                entry.schema.type === 'float' ? 4 : 0
                                            "
                                            :step="
                                                entry.schema.type === 'float' ? 0.01 : 1
                                            "
                                            :disabled="!isEditable(entry.schema)"
                                        />
                                    </template>
                                    <template v-else>
                                        <n-input
                                            v-model:value="draftValues[entry.schema.key]"
                                            :type="
                                                useTextarea(entry.schema)
                                                    ? 'textarea'
                                                    : 'text'
                                            "
                                            :autosize="
                                                useTextarea(entry.schema)
                                                    ? { minRows: 3, maxRows: 6 }
                                                    : undefined
                                            "
                                            :disabled="!isEditable(entry.schema)"
                                            :placeholder="t('adminSettings.placeholder.inputValue')"
                                        />
                                    </template>
                                </div>

                                <div class="setting-hint">
                                    {{ editorHintText(entry) }}
                                </div>
                            </div>
                        </n-card>
                    </div>

                    <n-collapse
                        v-if="inactiveDomains.length > 0"
                        class="inactive-collapse"
                    >
                        <n-collapse-item
                            :title="t('adminSettings.inactiveTitle', { count: inactiveItemCount })"
                            name="inactive"
                        >
                            <div
                                v-for="group in inactiveDomains"
                                :key="`inactive-${group.key}`"
                                class="inactive-group"
                            >
                                <div class="inactive-group-title">{{ group.label }}</div>
                                <div
                                    v-for="section in group.sections"
                                    :key="`inactive-${group.key}-${section.key}`"
                                    class="inactive-section"
                                >
                                    <div class="inactive-section-title">
                                        {{ section.label }}
                                    </div>
                                    <div
                                        v-for="entry in section.items"
                                        :key="`inactive-${entry.schema.key}`"
                                        class="inactive-item"
                                    >
                                        <div class="inactive-item-top">
                                            <div>
                                                <div class="setting-item-title">
                                                    {{ entry.schema.label }}
                                                </div>
                                                <div class="setting-item-key">
                                                    {{ entry.schema.key }}
                                                </div>
                                            </div>
                                            <n-space size="small">
                                                <n-tag round size="small" type="default"
                                                    >{{ t('adminSettings.tag.inactive') }}</n-tag
                                                >
                                                <n-tag
                                                    round
                                                    size="small"
                                                    :type="
                                                        applyModeTagType(
                                                            entry.schema.apply_mode
                                                        )
                                                    "
                                                >
                                                    {{
                                                        applyModeLabel(
                                                            entry.schema.apply_mode
                                                        )
                                                    }}
                                                </n-tag>
                                                <n-tag
                                                    v-if="entry.schema.secret"
                                                    round
                                                    size="small"
                                                    >{{ t('adminSettings.tag.secret') }}</n-tag
                                                >
                                            </n-space>
                                        </div>
                                        <div class="setting-item-description">
                                            {{ entry.schema.description }}
                                        </div>
                                        <div class="setting-meta inactive-meta">
                                            <span class="meta-item">
                                                {{ t('adminSettings.label.currentStatus') }}{{ currentStatusText(entry) }}
                                            </span>
                                            <span class="meta-item">
                                                {{ t('adminSettings.label.bootstrapBaseline') }}{{
                                                    bootstrapStatusText(entry.schema)
                                                }}
                                            </span>
                                        </div>
                                        <div class="setting-hint">
                                            {{ t('adminSettings.hint.inactiveSection') }}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </n-collapse-item>
                    </n-collapse>

                    <div class="form-submit-wrap">
                        <n-button
                            round
                            quaternary
                            :disabled="saving || loading"
                            @click="handleReset"
                        >
                            {{ t('adminSettings.action.resetUnsaved') }}
                        </n-button>
                        <n-button
                            round
                            type="primary"
                            secondary
                            :loading="saving"
                            @click="handleSave"
                        >
                            {{ t('adminSettings.action.saveConfig') }}
                        </n-button>
                    </div>
                </n-space>
            </n-spin>
        </n-card>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";
import { userInfo as fetchUserInfo } from "@/api/auth";
import { getSiteProfile } from "@/api/site";
import { useStoreMain } from "@/store/main";
import { useStoreProfile } from "@/store/profile";
import { TOKEN_KEY, useStoreUser } from "@/store/user";
import { Api } from "@/utils/request";
import { useI18n } from 'vue-i18n';
const { t } = useI18n();

type SettingPrimitive = string | number | boolean | null;
type SchemaItem = Api.Admin.NetReq.SettingsSchemaItem;
type ValueItem = Api.Admin.NetReq.SettingsValueItem;
type ViewItem = {
    schema: SchemaItem;
    value?: ValueItem;
};
type GroupedSection = {
    key: string;
    label: string;
    items: ViewItem[];
};
type GroupedDomain = {
    key: string;
    label: string;
    sections: GroupedSection[];
    itemCount: number;
};

const groupLabelMap = computed<Record<string, string>>(() => ({
    web: t('adminSettings.group.web'),
    app: t('adminSettings.group.app'),
    search: t('adminSettings.group.search'),
    storage: t('adminSettings.group.storage'),
    notifications: t('adminSettings.group.notifications'),
    payments: t('adminSettings.group.payments'),
}));

const sectionLabelMap = computed<Record<string, string>>(() => ({
    profile: t('adminSettings.section.profile'),
    general: t('adminSettings.section.general'),
    limits: t('adminSettings.section.limits'),
    bridge: t('adminSettings.section.bridge'),
    meili: t('adminSettings.section.meili'),
    zinc: t('adminSettings.section.zinc'),
    common: t('adminSettings.section.common'),
    local_oss: t('adminSettings.section.localOss'),
    minio: t('adminSettings.section.minio'),
    s3: t('adminSettings.section.s3'),
    alioss: t('adminSettings.section.alioss'),
    cos: t('adminSettings.section.cos'),
    huawei_obs: t('adminSettings.section.huaweiObs'),
    sms_juhe: t('adminSettings.section.smsJuhe'),
    alipay: t('adminSettings.section.alipay'),
}));

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const { userInfo } = storeToRefs(storeUser);
const router = useRouter();

const loading = ref(false);
const saving = ref(false);
const hasPendingRestart = ref(false);
const schemaItems = ref<SchemaItem[]>([]);
const valueItems = ref<ValueItem[]>([]);
const draftValues = reactive<Record<string, SettingPrimitive>>({});
const initialDraftValues = reactive<Record<string, SettingPrimitive>>({});
const secretDraftValues = reactive<Record<string, string>>({});

const valueMap = computed(() => {
    const map: Record<string, ValueItem> = {};
    for (const item of valueItems.value) {
        map[item.key] = item;
    }
    return map;
});

const activeDomains = computed(() => buildDomains(true));
const inactiveDomains = computed(() => buildDomains(false));
const inactiveItemCount = computed(() => {
    return inactiveDomains.value.reduce((count, domain) => count + domain.itemCount, 0);
});

const buildDomains = (active: boolean): GroupedDomain[] => {
    const domains: GroupedDomain[] = [];
    const domainMap: Record<string, GroupedDomain> = {};

    for (const schema of schemaItems.value) {
        if (schema.active !== active) {
            continue;
        }

        if (!domainMap[schema.group]) {
            domainMap[schema.group] = {
                key: schema.group,
                label: groupLabel(schema.group),
                sections: [],
                itemCount: 0,
            };
            domains.push(domainMap[schema.group]);
        }

        const domain = domainMap[schema.group];
        let section = domain.sections.find((item) => item.key === schema.section);
        if (!section) {
            section = {
                key: schema.section,
                label: sectionLabel(schema.section),
                items: [],
            };
            domain.sections.push(section);
        }

        section.items.push({
            schema,
            value: valueMap.value[schema.key],
        });
        domain.itemCount++;
    }

    return domains;
};

const replaceRecord = <T>(target: Record<string, T>, next: Record<string, T>) => {
    for (const key of Object.keys(target)) {
        delete target[key];
    }
    for (const [key, value] of Object.entries(next)) {
        target[key] = value;
    }
};

const prettifyKey = (value: string) => {
    return value
        .split("_")
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
};

const groupLabel = (group: string) => {
    return groupLabelMap[group] ?? prettifyKey(group);
};

const sectionLabel = (section: string) => {
    return sectionLabelMap[section] ?? prettifyKey(section);
};

const isEditable = (schema: SchemaItem) => {
    return schema.active && !schema.readonly && schema.apply_mode !== "bootstrap_only";
};

const hasOptions = (schema: SchemaItem) => {
    return !!schema.options && schema.options.length > 0;
};

const settingOptions = (schema: SchemaItem) => {
    return schema.options ?? [];
};

const sourceLabel = (source?: string) => {
    return source === "override" ? t('adminSettings.source.override') : t('adminSettings.source.bootstrap');
};

const sourceTagType = (source?: string) => {
    return source === "override" ? "success" : "default";
};

const applyModeLabel = (applyMode: SchemaItem["apply_mode"]) => {
    switch (applyMode) {
        case "live":
            return t('adminSettings.mode.live');
        case "restart_required":
            return t('adminSettings.mode.restartRequired');
        case "bootstrap_only":
            return t('adminSettings.mode.bootstrapOnly');
        default:
            return applyMode;
    }
};

const applyModeTagType = (applyMode: SchemaItem["apply_mode"]) => {
    switch (applyMode) {
        case "live":
            return "success";
        case "restart_required":
            return "warning";
        case "bootstrap_only":
            return "default";
        default:
            return "default";
    }
};

const fallbackValueForType = (schema: SchemaItem): SettingPrimitive => {
    switch (schema.type) {
        case "bool":
            return false;
        case "int":
        case "float":
            return null;
        default:
            return "";
    }
};

const normalizeDraftValue = (
    schema: SchemaItem,
    value: SettingPrimitive
): SettingPrimitive => {
    if (value === undefined) {
        return fallbackValueForType(schema);
    }

    switch (schema.type) {
        case "bool":
            return Boolean(value);
        case "int":
        case "float":
            if (value === null || value === "") {
                return null;
            }
            return typeof value === "number" ? value : Number(value);
        default:
            return typeof value === "string" ? value : String(value ?? "");
    }
};

const extractDraftValue = (schema: SchemaItem): SettingPrimitive => {
    const current = valueMap.value[schema.key];
    if (schema.secret) {
        return fallbackValueForType(schema);
    }
    if (current?.value !== undefined) {
        return normalizeDraftValue(schema, current.value as SettingPrimitive);
    }
    if (current?.effective_value !== undefined) {
        return normalizeDraftValue(schema, current.effective_value as SettingPrimitive);
    }
    if (schema.bootstrap_value !== undefined) {
        return normalizeDraftValue(schema, schema.bootstrap_value as SettingPrimitive);
    }
    return fallbackValueForType(schema);
};

const rebuildDraftState = () => {
    const nextDrafts: Record<string, SettingPrimitive> = {};
    const nextInitialDrafts: Record<string, SettingPrimitive> = {};
    const nextSecretDrafts: Record<string, string> = {};

    for (const schema of schemaItems.value) {
        if (schema.secret) {
            nextSecretDrafts[schema.key] = "";
            continue;
        }

        const draftValue = extractDraftValue(schema);
        nextDrafts[schema.key] = draftValue;
        nextInitialDrafts[schema.key] = draftValue;
    }

    replaceRecord(draftValues, nextDrafts);
    replaceRecord(initialDraftValues, nextInitialDrafts);
    replaceRecord(secretDraftValues, nextSecretDrafts);
};

const formatValue = (value: unknown, schema?: SchemaItem) => {
    if (value === null || value === undefined) {
        return t('adminSettings.value.notSet');
    }
    if (typeof value === "boolean") {
        return value ? t('adminSettings.value.enabled') : t('adminSettings.value.disabled');
    }
    if (typeof value === "number") {
        if (schema?.type === "float") {
            return `${value}`;
        }
        return `${value}`;
    }
    const text = String(value).trim();
    return text.length > 0 ? text : t('adminSettings.value.notSet');
};

const configuredText = (configured?: boolean) => {
    return configured ? t('adminSettings.status.configured') : t('adminSettings.status.notConfigured');
};

const currentConfiguredValue = (entry: ViewItem) => {
    if (entry.value?.value !== undefined) {
        return entry.value.value;
    }
    if (entry.value?.effective_value !== undefined) {
        return entry.value.effective_value;
    }
    if (entry.schema.bootstrap_value !== undefined) {
        return entry.schema.bootstrap_value;
    }
    return null;
};

const currentStatusText = (entry: ViewItem) => {
    if (entry.schema.secret) {
        return configuredText(
            entry.value?.configured ?? entry.schema.bootstrap_configured
        );
    }
    return formatValue(currentConfiguredValue(entry), entry.schema);
};

const bootstrapStatusText = (schema: SchemaItem) => {
    if (schema.secret) {
        return configuredText(schema.bootstrap_configured);
    }
    return formatValue(schema.bootstrap_value, schema);
};

const showEffectiveValue = (entry: ViewItem) => {
    return (
        !entry.schema.secret &&
        entry.value?.pending_restart &&
        entry.value.effective_value !== undefined &&
        entry.value.value !== undefined &&
        entry.value.effective_value !== entry.value.value
    );
};

const useTextarea = (schema: SchemaItem) => {
    return schema.key.includes("private_key");
};

const editorHintText = (entry: ViewItem) => {
    if (!entry.schema.active) {
        return t('adminSettings.hint.inactiveItem');
    }
    if (entry.schema.secret) {
        if (!isEditable(entry.schema)) {
            return t('adminSettings.hint.secretReadonly');
        }
        return t('adminSettings.hint.secretEditable');
    }
    if (entry.schema.apply_mode === "bootstrap_only" || entry.schema.readonly) {
        return t('adminSettings.hint.bootstrapOnly');
    }
    if (entry.value?.pending_restart) {
        return t('adminSettings.hint.pendingRestart');
    }
    if (entry.schema.apply_mode === "restart_required") {
        return t('adminSettings.hint.restartRequired');
    }
    return t('adminSettings.hint.live');
};

const refreshPublicSiteProfile = async (updatedKeys: string[]) => {
    if (!updatedKeys.some((key) => key.startsWith("web_profile."))) {
        return;
    }

    try {
        const profile = await getSiteProfile();
        storeProfile.updateSiteProfile(profile);
    } catch (_err) {
        // do nothing
    }
};

const loadSettings = async () => {
    loading.value = true;
    try {
        const [schemaResp, valuesResp] = await Promise.all([
            Api.v1.admin.get.settings.schema(),
            Api.v1.admin.get.settings.values(),
        ]);
        schemaItems.value = schemaResp.items;
        valueItems.value = valuesResp.items;
        hasPendingRestart.value = valuesResp.has_pending_restart;
        rebuildDraftState();
    } catch (_err) {
        // do nothing
    } finally {
        loading.value = false;
    }
};

const ensureAdminAccess = async () => {
    if (!localStorage.getItem(TOKEN_KEY) && userInfo.value.id === 0) {
        storeMain.triggerAuth(true);
        storeMain.triggerAuthKey("signin");
        router.replace({
            name: "home",
        });
        return false;
    }

    if (userInfo.value.id === 0) {
        try {
            const currentUser = await fetchUserInfo();
            storeUser.updateUserinfo(currentUser);
        } catch (_err) {
            storeUser.userLogout();
            router.replace({
                name: "home",
            });
            return false;
        }
    }

    if (!userInfo.value.is_admin) {
        router.replace({
            name: "404",
        });
        return false;
    }

    return true;
};

const collectChangedItems = (): Api.Admin.NetParams.SettingValueInput[] | null => {
    normalizeDependentDrafts();

    const items: Api.Admin.NetParams.SettingValueInput[] = [];

    for (const schema of schemaItems.value) {
        if (!isEditable(schema)) {
            continue;
        }

        if (schema.secret) {
            const secretValue = (secretDraftValues[schema.key] ?? "").trim();
            if (secretValue.length > 0) {
                items.push({
                    key: schema.key,
                    value: secretValue,
                });
            }
            continue;
        }

        const nextValue = normalizeDraftValue(schema, draftValues[schema.key]);
        const initialValue = normalizeDraftValue(schema, initialDraftValues[schema.key]);

        if (
            (schema.type === "int" || schema.type === "float") &&
            (nextValue === null || Number.isNaN(nextValue))
        ) {
            window.$message.warning(t('adminSettings.msg.invalidNumber', { label: schema.label }));
            return null;
        }

        if (schema.type === "string") {
            const nextText = String(nextValue ?? "").trim();
            const initialText = String(initialValue ?? "").trim();
            if (nextText !== initialText) {
                items.push({
                    key: schema.key,
                    value: nextText,
                });
            }
            continue;
        }

        if (nextValue !== initialValue) {
            items.push({
                key: schema.key,
                value: nextValue as string | number | boolean,
            });
        }
    }

    return items;
};

const numericDraftValue = (key: string): number | null => {
    const schema = schemaItems.value.find((item) => item.key === key);
    if (!schema) {
        return null;
    }
    const value = normalizeDraftValue(schema, draftValues[key]);
    if (typeof value !== "number" || Number.isNaN(value)) {
        return null;
    }
    return value;
};

const setNumericDraftIfPresent = (key: string, nextValue: number) => {
    if (!(key in draftValues)) {
        return;
    }
    draftValues[key] = nextValue;
};

const normalizeDependentDrafts = () => {
    const maxTweetLength = numericDraftValue("web_profile.default_tweet_max_length");
    if (maxTweetLength !== null) {
        const webEllipsis = numericDraftValue("web_profile.tweet_web_ellipsis_size");
        const mobileEllipsis = numericDraftValue("web_profile.tweet_mobile_ellipsis_size");
        if (webEllipsis !== null && webEllipsis > maxTweetLength) {
            setNumericDraftIfPresent("web_profile.tweet_web_ellipsis_size", maxTweetLength);
        }
        if (mobileEllipsis !== null && mobileEllipsis > maxTweetLength) {
            setNumericDraftIfPresent("web_profile.tweet_mobile_ellipsis_size", maxTweetLength);
        }
    }

    const maxPageSize = numericDraftValue("app.max_page_size");
    const defaultPageSize = numericDraftValue("app.default_page_size");
    if (maxPageSize !== null && defaultPageSize !== null && defaultPageSize > maxPageSize) {
        setNumericDraftIfPresent("app.default_page_size", maxPageSize);
    }
};

const handleReset = () => {
    rebuildDraftState();
};

const handleSave = async (e: MouseEvent) => {
    e.preventDefault();

    const items = collectChangedItems();
    if (!items) {
        return;
    }
    if (items.length === 0) {
        window.$message.info(t('adminSettings.msg.noChanges'));
        return;
    }

    saving.value = true;
    try {
        const resp = await Api.v1.admin.post.settings.save({ items });
        valueItems.value = resp.items;
        hasPendingRestart.value = resp.has_pending_restart;
        rebuildDraftState();
        await refreshPublicSiteProfile(resp.updated_keys);
        window.$message.success(t('adminSettings.msg.saveSuccess', { count: resp.updated_keys.length }));
    } catch (_err) {
        // do nothing
    } finally {
        saving.value = false;
    }
};

onMounted(async () => {
    const allowed = await ensureAdminAccess();
    if (!allowed) {
        return;
    }
    await loadSettings();
});
</script>

<style lang="less" scoped>
.setting-card {
    margin-top: -1px;
    border-radius: 0;

    .settings-layout {
        width: 100%;
    }

    .setting-alert {
        margin-bottom: 0;
    }

    .domain-block {
        display: flex;
        flex-direction: column;
        gap: 12px;

        .domain-header {
            display: flex;
            justify-content: space-between;
            align-items: center;

            .domain-title {
                font-size: 16px;
                font-weight: 600;
            }

            .domain-subtitle {
                font-size: 12px;
                opacity: 0.7;
                margin-top: 4px;
            }
        }
    }

    .section-card {
        border-radius: 12px;

        .section-header {
            display: flex;
            justify-content: space-between;
            align-items: baseline;

            .section-title {
                font-weight: 600;
            }

            .section-subtitle {
                font-size: 12px;
                opacity: 0.7;
            }
        }
    }

    .setting-item,
    .inactive-item {
        padding: 8px 0 18px;
        border-bottom: 1px solid rgba(127, 127, 127, 0.12);

        &:last-child {
            padding-bottom: 0;
            border-bottom: none;
        }

        .setting-item-top,
        .inactive-item-top {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            gap: 12px;
        }

        .setting-item-title {
            font-size: 15px;
            font-weight: 600;
        }

        .setting-item-key {
            margin-top: 4px;
            font-size: 12px;
            opacity: 0.55;
            word-break: break-all;
        }

        .setting-item-description {
            margin-top: 8px;
            font-size: 13px;
            line-height: 1.7;
            opacity: 0.82;
        }

        .setting-meta {
            display: flex;
            flex-wrap: wrap;
            gap: 8px 16px;
            margin-top: 10px;

            .meta-item {
                font-size: 12px;
                opacity: 0.72;
            }
        }

        .setting-editor {
            margin-top: 12px;

            .number-input {
                width: 220px;
            }
        }

        .setting-hint {
            margin-top: 8px;
            font-size: 12px;
            line-height: 1.7;
            opacity: 0.72;
        }
    }

    .inactive-collapse {
        margin-top: 4px;

        .inactive-group {
            display: flex;
            flex-direction: column;
            gap: 16px;
        }

        .inactive-group-title {
            font-size: 15px;
            font-weight: 600;
        }

        .inactive-section {
            display: flex;
            flex-direction: column;
            gap: 12px;
        }

        .inactive-section-title {
            font-size: 13px;
            font-weight: 600;
            opacity: 0.75;
        }

        .inactive-meta {
            margin-top: 8px;
        }
    }

    .empty-wrap {
        padding: 40px 0;
    }

    .form-submit-wrap {
        display: flex;
        justify-content: flex-end;
        gap: 12px;
        padding-top: 4px;
    }
}

.dark {
    .setting-card {
        background-color: rgba(16, 16, 20, 0.75);
    }

    .section-card {
        background-color: #18181c;
    }
}
</style>
