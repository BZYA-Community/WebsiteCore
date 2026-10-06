<template>
  <div>
    <main-nav :title="t('course.list.title')" />
    <main class="catalog-page">
      <header class="catalog-header">
        <div><h1>{{ t('courseCatalog.title') }}</h1><p>{{ t('courseCatalog.description') }}</p></div>
        <n-button v-if="canCreate" :type="managing ? 'primary' : 'default'" :secondary="managing" @click="toggleManaging">{{ managing ? t('courseCatalog.finishManaging') : t('courseCatalog.manage') }}</n-button>
      </header>
      <form class="catalog-search" role="search" @submit.prevent="search">
        <n-input v-model:value="keyword" :input-props="{ 'aria-label': t('course.list.searchPlaceholder') }" :placeholder="t('course.list.searchPlaceholder')" clearable @clear="clearSearch" />
        <n-button attr-type="submit" :loading="loading">{{ t('common.search') }}</n-button>
      </form>
      <nav class="major-nav" :aria-label="t('courseCatalog.majorCategories')">
        <router-link :to="categoryLink(0)" :aria-current="!selectedCategory && !appliedKeyword ? 'page' : undefined" :class="{ active: !selectedCategory && !appliedKeyword }">{{ t('courseCatalog.allCategories') }}</router-link>
        <router-link v-for="group in roots" :key="group.id" :to="categoryLink(group.id)" :class="{ active: selectedPath[0]?.id === group.id }" :aria-current="selectedPath[0]?.id === group.id ? 'page' : undefined">{{ group.name }}</router-link>
      </nav>
      <div v-if="managing" class="management-bar">
        <span>{{ t('courseCatalog.managementHint') }}</span>
        <n-space>
          <n-button v-if="canManageGroups" @click="editGroup()">{{ selectedGroup ? t('courseCatalog.addSubcategory') : t('courseCatalog.addMajor') }}</n-button>
          <n-button v-if="groups.length" type="primary" @click="editCourse()">{{ t('course.list.createCourse') }}</n-button>
        </n-space>
      </div>
      <div v-if="groupsLoading" class="catalog-status" role="status"><n-spin /><span>{{ t('courseCatalog.loading') }}</span></div>
      <div v-else-if="groupsFailed" class="catalog-status"><p>{{ t('courseCatalog.loadFailed') }}</p><n-button @click="loadGroups">{{ t('courseCatalog.retry') }}</n-button></div>
      <template v-else>
        <nav v-if="selectedCategory || appliedKeyword" class="catalog-breadcrumb" :aria-label="t('courseCatalog.breadcrumb')">
          <router-link :to="categoryLink(0)">{{ t('courseCatalog.title') }}</router-link>
          <template v-for="group in selectedPath" :key="group.id"><span aria-hidden="true">/</span><router-link :to="categoryLink(group.id)">{{ group.name }}</router-link></template>
          <template v-if="appliedKeyword"><span aria-hidden="true">/</span><span>{{ t('courseCatalog.searchResults') }}</span></template>
        </nav>
        <div v-if="selectedCategory && !selectedGroup" class="catalog-status"><p>{{ t('courseCatalog.categoryMissing') }}</p><router-link :to="categoryLink(0)">{{ t('courseCatalog.allCategories') }}</router-link></div>
        <template v-else-if="!selectedCategory && !appliedKeyword && !focusedCourseId">
          <section v-for="group in roots" :key="group.id" class="major-section">
            <header class="section-heading">
              <div><h2><router-link :to="categoryLink(group.id)">{{ group.name }}</router-link></h2><span>{{ t('courseCatalog.courseCount', { count: subtreeCount(group.id) }) }}</span></div>
              <div v-if="managing && canManageGroups" class="category-actions">
                <n-button size="small" @click="editGroup(undefined, group.id)">{{ t('courseCatalog.addSubcategory') }}</n-button>
                <n-button size="small" @click="editGroup(group)">{{ t('common.edit') }}</n-button>
                <n-popconfirm @positive-click="removeGroup(group)"><template #trigger><n-button size="small" quaternary type="error">{{ t('common.delete') }}</n-button></template>{{ t('course.list.deleteGroupConfirm', { name: group.name }) }}</n-popconfirm>
              </div>
            </header>
            <div class="subcategory-grid">
              <router-link v-for="child in childrenOf(group.id)" :key="child.id" :to="categoryLink(child.id)" class="subcategory-link">
                <span><strong>{{ child.name }}</strong><small>{{ t('courseCatalog.courseCount', { count: subtreeCount(child.id) }) }}</small></span><n-icon size="20" aria-hidden="true"><chevron-forward-outline /></n-icon>
              </router-link>
              <router-link v-for="item in directCourses(group.id).slice(0, 4)" :key="'course-' + item.id" :to="{ ...categoryLink(group.id), query: { ...categoryLink(group.id).query, course: String(item.id) } }" class="subcategory-link">
                <span><strong>{{ item.title }}</strong><small>{{ t('courseCatalog.viewLessons') }}</small></span><n-icon size="20" aria-hidden="true"><chevron-forward-outline /></n-icon>
              </router-link>
              <router-link v-if="group.course_count > Math.min(directCourses(group.id).length, 4) || (!group.course_count && !childrenOf(group.id).length)" :to="categoryLink(group.id)" class="subcategory-link">
                <span><strong>{{ t('courseCatalog.viewLessons') }}</strong><small>{{ group.name }}</small></span><n-icon size="20" aria-hidden="true"><chevron-forward-outline /></n-icon>
              </router-link>
            </div>
          </section>
          <div v-if="!roots.length" class="catalog-status"><p>{{ t('course.list.noGroups') }}</p><p>{{ managing ? t('courseCatalog.createFirstCategory') : t('courseCatalog.comingSoon') }}</p></div>
        </template>
        <template v-else>
          <header class="selection-heading">
            <div><h2>{{ appliedKeyword ? t('courseCatalog.searchFor', { keyword: appliedKeyword }) : focusedCourseId ? (courses[0]?.title || t('courseCatalog.viewLessons')) : selectedGroup?.name }}</h2><p>{{ t('courseCatalog.chooseLesson') }}</p></div>
            <div v-if="managing && selectedGroup && canManageGroups" class="category-actions">
              <n-button size="small" @click="editGroup(selectedGroup)">{{ t('course.list.editGroup') }}</n-button>
              <n-popconfirm @positive-click="removeGroup(selectedGroup)"><template #trigger><n-button size="small" quaternary type="error">{{ t('common.delete') }}</n-button></template>{{ t('course.list.deleteGroupConfirm', { name: selectedGroup.name }) }}</n-popconfirm>
            </div>
          </header>
          <nav v-if="childGroups.length && !appliedKeyword && !focusedCourseId" class="subcategory-grid selection-children" :aria-label="t('courseCatalog.subcategories')">
            <router-link v-for="child in childGroups" :key="child.id" :to="categoryLink(child.id)" class="subcategory-link">
              <span><strong>{{ child.name }}</strong><small>{{ t('courseCatalog.courseCount', { count: subtreeCount(child.id) }) }}</small></span><n-icon size="20" aria-hidden="true"><chevron-forward-outline /></n-icon>
            </router-link>
          </nav>
          <nav v-else-if="siblings.length > 1 && !appliedKeyword && !focusedCourseId" class="sibling-nav" :aria-label="t('courseCatalog.subcategories')">
            <router-link v-for="sibling in siblings" :key="sibling.id" :to="categoryLink(sibling.id)" :class="{ active: sibling.id === selectedCategory }" :aria-current="sibling.id === selectedCategory ? 'page' : undefined">{{ sibling.name }}</router-link>
          </nav>
          <template v-if="showOutlines">
            <div v-if="loading" class="catalog-status" role="status"><n-spin /><span>{{ t('courseCatalog.loading') }}</span></div>
            <div v-else-if="listFailed" class="catalog-status"><p>{{ t('courseCatalog.loadFailed') }}</p><n-button @click="loadCourses">{{ t('courseCatalog.retry') }}</n-button></div>
            <template v-else>
              <article v-for="course in courses" :id="'course-' + course.id" :key="course.id" class="course-outline">
                <header class="outline-heading">
                  <div>
                    <router-link v-if="appliedKeyword" class="course-category" :to="categoryLink(course.group_id)">{{ pathFor(course.group_id).map(group => group.name).join(' / ') }}</router-link>
                    <h3>{{ course.title }}</h3>
                    <p class="course-teacher">{{ t('course.detail.teacher') }}：{{ course.teacher?.nickname || t('course.card.unknownTeacher') }}</p>
                  </div>
                  <div v-if="managing && canManageCourse(course)" class="category-actions">
                    <n-button size="small" @click="editCourse(course)">{{ t('course.list.editCourse') }}</n-button>
                    <n-popconfirm @positive-click="removeCourse(course)"><template #trigger><n-button size="small" quaternary type="error">{{ t('common.delete') }}</n-button></template>{{ t('course.card.deleteConfirm', { title: course.title }) }}</n-popconfirm>
                  </div>
                </header>
                <p v-if="course.intro" class="outline-intro">{{ course.intro }}</p>
                <course-lessons :course="course" :managing="managing" />
              </article>
              <div v-if="!courses.length" class="catalog-status">
                <template v-if="focusedCourseId && !storeUser.hasPermission('course.view')"><p>{{ storeUser.userLogined ? t('courseEditor.verifyHelp') : t('courseEditor.signInHelp') }}</p><router-link v-if="storeUser.userLogined" :to="{ name: 'setting' }">{{ t('courseEditor.verifyContact') }}</router-link><n-button v-else @click="signIn">{{ t('course.action.login') }}</n-button></template>
                <template v-else><p>{{ appliedKeyword ? t('course.list.noSearchResult') : t('courseCatalog.emptyCategory') }}</p><n-button v-if="managing" @click="editCourse()">{{ t('course.list.createCourse') }}</n-button></template>
              </div>
              <n-pagination v-if="total > pageSize" :page="page" :page-size="pageSize" :item-count="total" :page-slot="5" class="pagination" @update:page="changePage" />
            </template>
          </template>
        </template>
      </template>
    </main>
    <n-modal v-model:show="groupEditor" preset="card" :title="groupDraft.id ? t('course.list.editGroup') : t('course.list.createGroup')" style="width: min(460px, calc(100vw - 32px))" :closable="!savingGroup" :mask-closable="!savingGroup" :close-on-esc="!savingGroup">
      <n-form label-placement="top" @submit.prevent="saveGroup">
        <n-form-item :label="t('course.list.groupName')" required><n-input v-model:value="groupDraft.name" :input-props="{ 'aria-label': t('course.list.groupName') }" :maxlength="64" /></n-form-item>
        <n-form-item :label="t('course.list.parentCategory')"><n-select v-model:value="groupDraft.parent_id" :aria-label="t('course.list.parentCategory')" :options="parentOptions" /></n-form-item>
        <n-form-item :label="t('course.list.sortOrder')"><n-input-number v-model:value="groupDraft.sort" :min="0" :precision="0" /></n-form-item>
        <n-button attr-type="submit" type="primary" :loading="savingGroup">{{ t('common.save') }}</n-button>
      </n-form>
    </n-modal>
    <n-modal v-model:show="courseEditor" preset="card" :title="courseDraft.id ? t('course.list.editCourse') : t('course.list.createCourse')" style="width: min(640px, calc(100vw - 32px)); max-height: calc(100dvh - 32px); overflow-y: auto" :closable="!savingCourse" :mask-closable="!savingCourse" :close-on-esc="!savingCourse">
      <n-form label-placement="top" @submit.prevent="saveCourse">
        <n-form-item :label="t('course.list.belongGroup')" required><n-select v-model:value="courseDraft.group_id" :aria-label="t('course.list.belongGroup')" :options="categoryOptions" :placeholder="t('course.list.selectGroup')" /></n-form-item>
        <n-form-item :label="t('course.list.courseTitle')" required><n-input v-model:value="courseDraft.title" :input-props="{ 'aria-label': t('course.list.courseTitle') }" :maxlength="128" /></n-form-item>
        <n-form-item :label="t('course.list.courseIntro')"><n-input v-model:value="courseDraft.intro" :input-props="{ 'aria-label': t('course.list.courseIntro') }" type="textarea" :maxlength="2000" :autosize="{ minRows: 3, maxRows: 6 }" /></n-form-item>
        <n-form-item :label="t('course.list.courseTeacher')" required>
          <n-select v-model:value="courseDraft.teacher_id" :aria-label="t('course.list.courseTeacher')" :disabled="!storeUser.hasPermission('course.manage') || !storeUser.hasPermission('user.manage')" filterable remote :options="teacherOptions" :loading="teacherLoading" @search="searchTeachers" @focus="searchTeachers('')" />
        </n-form-item>
        <n-form-item :label="t('course.list.teacherIntro')"><n-input v-model:value="courseDraft.teacher_intro" :input-props="{ 'aria-label': t('course.list.teacherIntro') }" type="textarea" :maxlength="2000" :autosize="{ minRows: 2, maxRows: 5 }" /></n-form-item>
        <p>{{ t('courseCatalog.lessonHelp') }}</p>
        <n-button attr-type="submit" type="primary" :loading="savingCourse">{{ t('common.save') }}</n-button>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import { ChevronForwardOutline } from '@vicons/ionicons5';
import { useStoreUser } from '@/store/user';
import { useStoreMain } from '@/store/main';
import { Api } from '@/utils/request';
import CourseLessons from '@/components/course/course-lessons.vue';
import { getCourse, getCourseGroups, getCourseList, createCourseGroup, updateCourseGroup, deleteCourseGroup, createCourse, updateCourse, deleteCourse, type CourseGroup, type CourseItem } from '@/api/course';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const storeUser = useStoreUser();
const storeMain = useStoreMain();
function signIn() { storeMain.triggerAuthKey('signin'); storeMain.triggerAuth(true); }
const canCreate = computed(() => storeUser.hasAnyPermission(['course.manage', 'course.manage_own']));
const canManageGroups = computed(() => storeUser.hasPermission('course.manage'));
const managing = computed(() => canCreate.value && route.query.manage === '1');
const canManageCourse = (course: CourseItem) => canManageGroups.value || (storeUser.hasPermission('course.manage_own') && course.teacher_id === storeUser.userInfo.id);
const groups = ref<CourseGroup[]>([]);
const directoryCourses = ref<CourseItem[]>([]);
const directCourses = (id: number) => directoryCourses.value.filter(course => course.group_id === id);
const groupsLoading = ref(true);
const groupsFailed = ref(false);
const selectedCategory = computed(() => Number(route.query.category) || 0);
const focusedCourseId = computed(() => Number(route.query.course) || 0);
const selectedGroup = computed(() => groups.value.find(group => group.id === selectedCategory.value));
const roots = computed(() => groups.value.filter(group => !groups.value.some(parent => parent.id === group.parent_id)));
const childrenOf = (id: number) => groups.value.filter(group => group.parent_id === id);
const childGroups = computed(() => childrenOf(selectedCategory.value));
const siblings = computed(() => selectedGroup.value?.parent_id ? childrenOf(selectedGroup.value.parent_id) : []);
function pathFor(id: number) {
  const path: CourseGroup[] = [];
  const visited = new Set<number>();
  let group = groups.value.find(item => item.id === id);
  while (group && !visited.has(group.id)) {
    path.unshift(group); visited.add(group.id);
    group = groups.value.find(item => item.id === group!.parent_id);
  }
  return path;
}
const selectedPath = computed(() => pathFor(selectedCategory.value));
const subtreeCount = (id: number) => groups.value.reduce((sum, group) => sum + (pathFor(group.id).some(parent => parent.id === id) ? group.course_count : 0), 0);
const categoryOptions = computed(() => groups.value.map(group => ({ label: pathFor(group.id).map(parent => parent.name).join(' / '), value: group.id })));
const categoryLink = (id: number) => ({ name: 'courses', query: { ...(id ? { category: String(id) } : {}), ...(managing.value ? { manage: '1' } : {}) } });
function toggleManaging() { void router.replace({ query: { ...route.query, manage: managing.value ? undefined : '1' } }); }
async function loadGroups() {
  groupsLoading.value = true; groupsFailed.value = false;
  try {
    const [categories, preview] = await Promise.all([getCourseGroups(), getCourseList({ page: 1, page_size: 100 })]);
    directoryCourses.value = preview.list || [];
    groups.value = categories.groups || [];
  }
  catch { groupsFailed.value = true; }
  finally { groupsLoading.value = false; }
}
const appliedKeyword = computed(() => typeof route.query.q === 'string' ? route.query.q : '');
const keyword = ref(appliedKeyword.value);
const page = computed(() => Math.max(1, Math.floor(Number(route.query.page) || 1)));
const pageSize = 10;
const showOutlines = computed(() => Boolean(focusedCourseId.value || appliedKeyword.value || (selectedGroup.value && (!childGroups.value.length || selectedGroup.value.course_count))));
const courses = ref<CourseItem[]>([]);
const total = ref(0);
const loading = ref(false);
const listFailed = ref(false);
let listRevision = 0;
async function loadCourses() {
  const revision = ++listRevision;
  courses.value = []; total.value = 0; listFailed.value = false;
  if (!showOutlines.value) { loading.value = false; return; }
  loading.value = true;
  try {
    if (focusedCourseId.value) {
      const course = storeUser.hasPermission('course.view') ? (await getCourse({ id: focusedCourseId.value })).course : directoryCourses.value.find(item => item.id === focusedCourseId.value);
      if (revision !== listRevision) return;
      courses.value = course ? [course] : [];
      if (course && selectedCategory.value !== course.group_id) await router.replace({ query: { ...route.query, category: String(course.group_id) } });
      return;
    }
    const result = await getCourseList({ group_id: selectedCategory.value || undefined, keyword: appliedKeyword.value, page: page.value, page_size: pageSize });
    if (revision !== listRevision) return;
    courses.value = result.list || []; total.value = result.pager?.total_rows || 0;
  } catch { if (revision === listRevision) listFailed.value = true; }
  finally { if (revision === listRevision) loading.value = false; }
}
function search() { void router.push({ name: 'courses', query: { ...route.query, course: undefined, q: keyword.value.trim() || undefined, page: undefined } }); }
function clearSearch() { keyword.value = ''; search(); }
function changePage(value: number) { void router.push({ query: { ...route.query, page: value > 1 ? String(value) : undefined } }); }
watch(appliedKeyword, value => { keyword.value = value; });
watch([selectedCategory, focusedCourseId, appliedKeyword, page, showOutlines, () => storeUser.hasPermission('course.view')], () => { void loadCourses(); });
const groupEditor = ref(false);
const savingGroup = ref(false);
const groupDraft = reactive({ id: 0, name: '', parent_id: 0, sort: 0 });
const parentOptions = computed(() => [{ label: t('course.list.rootCategory'), value: 0 }, ...categoryOptions.value.filter(option => !pathFor(option.value).some(group => group.id === groupDraft.id))]);
function editGroup(group?: CourseGroup, parentID = selectedCategory.value) {
  Object.assign(groupDraft, { id: group?.id || 0, name: group?.name || '', parent_id: group?.parent_id ?? parentID, sort: group?.sort || 0 });
  groupEditor.value = true;
}
async function saveGroup() {
  if (savingGroup.value) return;
  if (!groupDraft.name.trim()) return window.$message.warning(t('course.list.inputGroupName'));
  savingGroup.value = true;
  try {
    const data = { ...groupDraft, name: groupDraft.name.trim(), sort: groupDraft.sort || 0 };
    if (data.id) await updateCourseGroup(data); else await createCourseGroup(data);
    groupEditor.value = false; await loadGroups(); window.$message.success(t('common.operationSuccess'));
  } catch { /* Retain the draft for retry. */ }
  finally { savingGroup.value = false; }
}
async function removeGroup(group: CourseGroup) {
  try { await deleteCourseGroup({ id: group.id }); if (selectedCategory.value === group.id) await router.replace(categoryLink(group.parent_id)); await loadGroups(); }
  catch { return false; }
}
const courseEditor = ref(false);
const savingCourse = ref(false);
const courseDraft = reactive({ id: 0, group_id: null as number | null, teacher_id: 0, title: '', intro: '', teacher_intro: '' });
const teacherOptions = ref<{ label: string; value: number }[]>([]);
const teacherLoading = ref(false);
async function searchTeachers(keyword: string) {
  if (!storeUser.hasPermission('user.manage')) return;
  teacherLoading.value = true;
  try { teacherOptions.value = (await Api.v1.admin.get.user.list({ keyword, page: 1, page_size: 20 })).list.map((user) => ({ label: `${user.nickname} (@${user.username})`, value: user.id })); }
  catch { /* Retain existing selections. */ }
  finally { teacherLoading.value = false; }
}
function editCourse(course?: CourseItem) {
  Object.assign(courseDraft, { id: course?.id || 0, group_id: course?.group_id || selectedCategory.value || null, teacher_id: course?.teacher_id || storeUser.userInfo.id, title: course?.title || '', intro: course?.intro || '', teacher_intro: course?.teacher_intro || '' });
  const teacher = course?.teacher || storeUser.userInfo;
  teacherOptions.value = [{ label: `${teacher.nickname} (@${teacher.username})`, value: courseDraft.teacher_id }];
  courseEditor.value = true;
}
async function saveCourse() {
  if (savingCourse.value) return;
  if (!courseDraft.group_id) return window.$message.warning(t('course.list.selectGroupRequired'));
  if (!courseDraft.title.trim()) return window.$message.warning(t('course.list.inputCourseTitle'));
  if (!courseDraft.teacher_id) return window.$message.warning(t('course.list.selectTeacherRequired'));
  savingCourse.value = true;
  try {
    const data = { ...courseDraft, group_id: courseDraft.group_id, title: courseDraft.title.trim() };
    if (data.id) await updateCourse(data); else await createCourse(data);
    courseEditor.value = false; await loadGroups(); await router.push({ name: 'courses', query: { category: String(data.group_id), manage: '1' } }); await loadCourses(); window.$message.success(t('common.operationSuccess'));
  } catch { /* Retain the draft for retry. */ }
  finally { savingCourse.value = false; }
}
async function removeCourse(course: CourseItem) {
  try {
    await deleteCourse({ id: course.id });
    courses.value = courses.value.filter(item => item.id !== course.id);
    if (focusedCourseId.value === course.id) await router.replace(categoryLink(course.group_id));
    await loadGroups(); await loadCourses(); window.$message.success(t('course.deleteSuccess'));
  }
  catch { return false; }
}
onMounted(async () => { await loadGroups(); await loadCourses(); });
</script>

<style scoped lang="less">
.catalog-page { box-sizing: border-box; padding: 16px; border: 1px solid var(--course-line); border-top: 0; color: var(--course-text); font-size: 14px; }
.catalog-header { display: flex; justify-content: space-between; align-items: center; gap: 16px; h1 { font-size: 18px; font-weight: 500; line-height: 1.5; margin: 0 0 6px; } p { margin: 0; color: var(--course-muted); line-height: 1.6; } }
.catalog-search { display: flex; gap: 8px; margin: 16px 0; .n-input { flex: 1; min-width: 0; } }
.major-nav, .sibling-nav { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px; a { padding: 4px 14px; border-radius: 16px; text-decoration: none; color: var(--course-muted); background: var(--course-hover); } a:hover, a.active { color: var(--course-accent); background: var(--course-tint); } }
.management-bar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; border-block: 1px solid var(--course-line); padding: 12px 0; margin-bottom: 16px; color: var(--course-muted); font-size: 13px; }
.major-section { padding: 16px 0; border-top: 1px solid var(--course-line); }
.section-heading, .selection-heading, .outline-heading { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; h2, h3 { margin: 0; font-size: 16px; font-weight: 500; line-height: 1.5; overflow-wrap: anywhere; } a { color: inherit; text-decoration: none; &:hover { color: var(--course-accent); } } }
.section-heading { margin-bottom: 12px; > div:first-child { display: flex; align-items: baseline; flex-wrap: wrap; gap: 10px; } span { color: var(--course-muted); font-size: 12px; } }
.subcategory-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(220px, 100%), 1fr)); gap: 8px; }
.subcategory-link { min-width: 0; display: flex; gap: 12px; align-items: center; justify-content: space-between; color: var(--course-text); text-decoration: none; padding: 12px; border-radius: var(--course-radius); border: 1px solid var(--course-line); &:hover { color: var(--course-accent); background: var(--course-hover); } strong { display: block; font-weight: 400; overflow-wrap: anywhere; } small { display: block; color: var(--course-muted); font-size: 12px; margin-top: 4px; } .n-icon { flex: 0 0 auto; color: var(--course-muted); } }
.catalog-breadcrumb { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; font-size: 13px; color: var(--course-muted); margin-bottom: 16px; a { color: inherit; text-decoration: none; &:hover { color: var(--course-accent); } } }
.selection-heading { margin-bottom: 16px; p { color: var(--course-muted); margin: 6px 0 0; } }
.selection-children { margin-bottom: 16px; }
.course-outline { border-top: 1px solid var(--course-line); padding-top: 16px; margin-bottom: 16px; scroll-margin-top: 80px; }
.course-outline :deep(.lesson-section) { padding-top: 8px; }
.course-teacher { font-size: 13px; color: var(--course-muted); margin: 6px 0 0; }
.outline-heading .course-category { display: inline-block; margin-bottom: 6px; color: var(--course-accent); font-size: 12px; }
.outline-intro { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.75; max-width: 75ch; color: var(--course-muted); margin: 12px 0 0; }
.category-actions { display: flex; flex-wrap: wrap; gap: 8px; flex-shrink: 0; }
.catalog-status { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 50px 20px; text-align: center; color: var(--course-muted); p { margin: 0; } a { color: var(--course-accent); } }
.pagination { margin-top: 16px; }
@media (max-width: 680px) { .catalog-header { align-items: flex-start; flex-wrap: wrap; } .section-heading, .selection-heading, .outline-heading { flex-wrap: wrap; } .subcategory-grid { grid-template-columns: 1fr; } }
</style>
