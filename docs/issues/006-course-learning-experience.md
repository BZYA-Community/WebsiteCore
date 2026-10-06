# Course catalog and lesson watching experience

Status: implemented and locally verified. Follow-up to Phase 2 after hands-on user feedback.

## Problem and accepted flow

The category tree, course cards, lesson accordion, and attachment playback dialog
made watching a lesson unnecessarily difficult. The learning path is now explicit:

```text
Course directory
  Major category
    Subcategory
      Course title (section heading, no extra navigation step)
        Lesson link
          Watch page: player | lesson playlist
                      lesson description / materials / questions
```

Use the existing category, course, and lesson records. Preserve teacher ownership,
resource permissions, verified uploads, and moderated questions. Category and lesson
selection belong in the URL so Back, refresh, and shared links remain meaningful.
Management is an explicit catalog mode; it does not obstruct watching.

Existing courses directly under a major category remain named entries (for
example, Machine learning → Supervised learning). They open their own lesson
outline, without requiring data to be moved into a newly invented category.
The directory previews up to four directly owned courses per category from the
latest 100 catalog records; an explicit directory link and paginated results
retain access to all remaining courses. Lesson resources are fetched only after
entering an outline with `course.view` permission.

## Design plan

- Palette: inherit the existing Naive UI light and dark theme tokens. Keep the
  site's green actions, white or dark surfaces, neutral text, and faint dividers;
  do not introduce a separate course color scheme.
- Type: inherit the site's existing Lato/v-sans font stack. Use 14px body and
  lesson text, with 16–18px headings. Descriptions stay below 80 characters per
  line; no course-specific font override or oversized page title.
- Layout: left-aligned category sections with named subcategory links; within a
  subcategory, course headings followed immediately by ordered lesson links.
  Preserve the original content column width, alignment, and sidebar positions.
  The directory keeps the community right sidebar; only the watch page hides it.
  A narrow content column places the playlist below the player.
- Principle: content hierarchy supplies the visual hierarchy. No decorative
  thumbnails, statistic banners, animated card grids, or attachment dialogs as
  the main video entry point. Use flat sections, faint dividers, and the site's
  small corner radii instead of large outlined cards. Lesson numbers represent
  actual sequence.

Review against the brief: a conventional course-card grid would still require an
extra click before choosing a lesson. The final plan uses the course title only
as a heading, exposing lessons directly inside the chosen subcategory. The
accepted navigation remains intact while the visual treatment follows the
existing site.

## Acceptance checks

- Follow major category → subcategory → lesson → watch without opening an editor.
- Change lessons, reload a deep link, and use Back without stale titles or videos.
- Play a real uploaded video and open a supporting file using signed resource URLs.
- Keep teacher introduction, lesson description, and course questions available.
- Distinguish loading, empty, permission, and failure states; provide retry paths.
- Edit a lesson's video and materials without dropping untouched attachments.
- Check mobile and desktop layouts, keyboard links, dark mode, and both languages.
- Compare the catalog's columns, sidebar positions, font, background, and active
  color against the original home page.
- Run frontend lint, locale parity, production build, and live browser checks.

## Verification and repeatable checks

The live checks use dedicated local sample content and real LocalOSS video and
text uploads. They exercise playback, previous/next lesson, signed downloads,
question moderation, named direct-course entries, keyboard tabs, and guest
deep-link guidance. Chinese/light and English/dark layouts were checked at
375, 390, 821, 822, 1000, 1366, and 1920 pixels as applicable. Narrow container layouts
put the playlist below the player, including when the desktop sidebar is visible.

The editor check replaces a real video, verifies the original handout remains,
simulates a failed save, and retries the retained draft. The replacement and
question submission apply only to the dedicated experience course.

From `web`, run:

```sh
npm run test:courses
npm run test:permissions
npm run lint
npm run i18n:check
npm run build
```

The component checks execute the actual Vue script setup using the installed
Vue/TypeScript runtimes and controllable requests. They cover ownership and
permission changes, multiple-video preservation, cancelled uploads, busy saves,
stale signed responses, question isolation, and failed-request retries.

For the live browser check, install Python Playwright and a browser on the test
machine. Windows defaults to installed Edge; other platforms default to
Playwright Chromium. `--browser-channel` can select another installed channel.
Keep credentials and synthetic fixture IDs outside Git. The account JSON contains
`MemberUsername` and `OperatorPassword` (the shared test password), plus
`OperatorUsername` when using `--video`. The fixture JSON contains `category`,
`course`, and `lessons` (two ordered objects with `id` and `title`). The dedicated
course needs two playable lessons, each with a video and `course-notes.txt`.

From the repository root:

```sh
python web/scripts/course-flow.test.py --base http://127.0.0.1:8008 --accounts custom/preview/accounts.json --fixtures custom/preview/course-ui/fixtures.json
```

Add `--video /path/to/sample.mp4` to exercise replacement on the isolated course.
This optional test requires its title to contain `体验课程`; it must never be
pointed at real teaching content. Both UI and API URLs must be loopback origins.
The script retains its sample question and edited lesson for manual inspection.

Production frontend and embedded Go builds pass. Repository ESLint reports zero
errors and 136 pre-existing warnings; locale keys have no missing entries and
Chinese/English parity holds. External OSS and mail services are outside this
frontend-only verification.

Query navigation follows the Vue Router guidance on watching the specific
[reactive route properties](https://router.vuejs.org/guide/advanced/composition-api.html)
that change when a component is reused.
