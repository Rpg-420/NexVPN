# وضعیت پروژه (Project Status)

- **نسخه فعلی:** `1.4.0` — منبع یکتای حقیقت: `version.go`
- **پلتفرم اصلی:** Windows (Go + Wails v2)، با پشتیبانی جزئی از Linux/macOS
- **آخرین ممیزی مخزن:** ۱۴۰۵/۰۷/۲۹ (اکتبر ۲۰۲۶)
- **آخرین به‌روزرسانی این سند:** پس از انتشار v1.4.0 (سخت‌سازی زنجیرهٔ تأمین هسته، بستر تست فرانت‌اند، بهداشت مخزن)

> این سند جای «گزارش ممیزی» را نمی‌گیرد. برای جزئیات کامل باگ‌ها، شدت و اولویت رفع،
> به `docs/PROJECT_AUDIT_REPORT_FA.md` مراجعه کنید. خلاصهٔ نتیجهٔ هر بخش و شواهد
> اعتبارسنجی در `docs/REMEDIATION_SUMMARY_FA.md` آمده است.

## ۰) بازبینی چهارم (۱۴۰۵/۰۷/۲۹) — گزارش کامل در گفتگو و `CHANGELOG_DEV.md`

- **سلامت فنی:** `go build`, `go vet`, `go test ./internal/...`, `tsc --noEmit`, `vitest run` همه سبز،
  و پس از پاس پاک‌سازی `golangci-lint` هم **صفر ایراد** دارد (بخش ۵). تنها نکتهٔ تاریخی:
  `TestSingBoxWireGuardEngineStartsWithoutDNSFatal` یک‌بار در اجرای کل پکیج شکست؛
  ریشه محدودیت bind یو‌دی‌پیWireGuard در محیط sandbox بود (تست کنترل و اجرای مجدد پاس شد) —
  باگ کد نبود.
- **رفع‌شده در این پاس:** پین SHA-256 باینری‌های هسته (قبلاً خالی بود)، پارس sidecar فرمت
  `.dgst` اکس‌ری (قبلاً `fields[0]` رشتهٔ `MD5=` را می‌خواند و همیشه رد می‌کرد)، بستر تست
  فرانت‌اند از صفر به ۲۶ تست، استخراج `prepareConnectionMode` از `Engine.Connect`.
- **تأیید already-fixed (ممیزی قدیمی کهنه بود):** Gaming Optimizer واقعاً از رجیستری ویندوز
  می‌خواند؛ `GetInstalledApplications` نصب واقعی را روی دیسک چک می‌کند؛ Psiphon با گزینه‌های
  پیشرفتهٔ واقعی اجرا می‌شود؛ `PanicWipe`/`RepairNetworkIssues` خطاهای واقعی برمی‌گردانند.
- **باقی‌ماندهٔ آگاه (follow-up، follow-up ثبت‌شده):**
  1. استخراج کامل goroutine اسپاون/فالبک از `Engine.Connect` با struct وضعیت (پس از انتشار).
  2. pin کردن هستهٔ Aether هنگام انتشار نسخهٔ بعدی (URL آن version-pinned است).
  3. مجازی‌سازی لیست پروفایل برای صدها پروفایل.

## ۱) یافته‌های ممیزی و وضعیت رفع

۴۵ یافته در چهار سطح شدت شناسایی شد. وضعیت نهایی:

| وضعیت | تعداد | موارد |
| --- | --- | --- |
| ✅ رفع کامل | ۴۲ | C1–C5، H1–H8، M1، M2، M4–M13، M15–M21، L1–L4، L6–L11 |
| 🟡 رفع جزئی | ۱ | M3 (باقی‌ماندهٔ `_ = ...` در Go عمدی و مستند است) |
| 🟠 کاهش ریسک | ۱ | L5 (اتکا به فلگ‌های deprecated در sing-box) |
| ✅ رفع کامل | ۱ | M14 (`dist/` اکنون تازه است و به CI واگذار شده) |

| فاز | دامنه | وضعیت |
| --- | --- | --- |
| فاز ۰ | رفع موانع بحرانی (index مخدوش گیت، رندر سه‌باره مودال‌ها، حلقه آمار غیرفعال، باگ Kill Switch) | ✅ انجام شد |
| فاز ۱ | امنیت و یکپارچگی (verify امضای باینری‌های هسته، کلید پیش‌فرض LAN sync) | ✅ انجام شد |
| فاز ۲ | صحت و i18n (حذف پارامترهای AmneziaWG از WARP، کلیدهای گم‌شده i18n، خطاهای متناقض OpenVPN، تشخیص‌های واقعی Doctor، بازه آپدیت اشتراک) | ✅ انجام شد |
| فاز ۳ | پاک‌سازی و مستندات (کد مرده، آلودگی مستندات، mojibake، فایل‌های زائد build، `strict: true`، مهاجرت کامل i18n) | ✅ انجام شد |
| فاز ۴ | مقاوم‌سازی، تست و CI | ✅ انجام شد |

### جزئیات فاز ۳

- **کد مرده حذف شد:** `internal/core/builder.go` (+ تست)، `HasAmneziaWGParameters`، `waitForTunIP`،
  پوشش اضافی `cleanupStaleCores`، محاسبهٔ دورریز `singboxJSON`، `frontend/src/pages/Profiles.tsx`،
  `frontend/src/components/Toast.tsx`.
- **مستندات:** ورودی‌های پروژهٔ بیگانه (RTK) از `CHANGELOG_DEV.md` و `RELEASE_NOTES.md` حذف شد؛
  ۱۱۱ خط mojibake در `RELEASE_NOTES.md` ترمیم شد؛ آلودگی سیریلیک/عربی پاک شد؛ `STATUS.md` بازنویسی شد.
- **آرتیفکت‌های build:** `bin/wxray.exe` (۲۸ مگابایت، بی‌استفاده) از مخزن حذف شد؛
  `build/bin/wintun.dll` از نسخهٔ ۳۲بیتی اشتباه به نسخهٔ صحیح ۶۴بیتی جایگزین شد.
- **TypeScript:** `strict: true` روشن شد و با `tsc --noEmit` بدون خطا تأیید شد؛ `@types/three` اضافه شد.
- **i18n:** ۳۸ کلید گم‌شده اضافه شد؛ `ServiceUnlockModal.tsx` کامل مهاجرت شد؛
  `notification.ts` و پیام‌های `alert` محلی‌سازی شدند.
- **i18n (تکمیل M2):** ۸۸ کلید جدید × ۲ زبان اضافه شد و **تمام** رشته‌های هاردکد فارسی
  از `Settings.tsx` و `SmartISPOptimizerCard.tsx` حذف شدند (۹۰ رشته).
  توازن کلیدهای `en`/`fa` اکنون ۱۰۶۱ به ۱۰۶۱ و بدون اختلاف است.
  فایل‌های `CountryFlag.tsx`، `CommandPalette.tsx` و `theme.ts` **عمداً** مهاجرت نشدند،
  چون دادهٔ عملکردی چندزبانه‌اند و i18n کردنشان جستجو/انتخاب تم را می‌شکند.

### جزئیات فاز ۴

- **M4:** آداپتور Xray حالا `exitCh`/`exitOnce` دارد و `Start()` با پروب زنده‌بودن ۱۵۰۰ms
  در صورت خروج زودهنگام هسته شکست می‌خورد؛ صفر بودن آمار بایتی عمدی و مستند شد
  (موتور به `QueryTunnelAdapterBytes` برمی‌گردد).
- **M5:** `QuotaMonitorDaemon` پس‌زمینه اضافه شد (`Start`/`Stop`/`SetInterval`/`CheckAll`)،
  در `NewApp`، `shutdown` و `SaveSettings` سیم‌کشی شد؛ `shouldAlert` با پنجرهٔ ۲۴ ساعته
  هشدارها را throttle می‌کند؛ `Store.GetSubscriptionURLs()` اضافه شد.
- **M6:** نشت `time.AfterFunc` رفع شد (تایمر ذخیره و `Stop` می‌شود)، کالبک «ناسالم» فقط
  یک‌بار در هر قطعی شلیک می‌شود، و `SetUnhealthyCallback` حالا واقعاً failover انجام می‌دهد
  (احترام به `AutoFailover`، سقف `defaultMaxFailoverAttempts = 3`).
- **M21:** سرور کنترل از راه دور حالا `idleTimeout` (پیش‌فرض `DefaultIdleTimeout = 30m`) و
  watchdog دارد و پس از بی‌فعالیتی خودکار خاموش می‌شود؛ هشدار افشای شبکهٔ محلی به صفحهٔ PIN
  و به `MobileRemoteModal` اضافه شد؛ `idle_timeout_sec` تا لایهٔ TypeScript منتقل شد.
- **M12:** مودال‌ها `role="dialog"` و backdropها `role="button" tabIndex={-1}` گرفتند؛
  دکمهٔ فقط-آیکون پاک‌کردن جستجو `aria-label` گرفت.
- **M13 (امنیت/حریم):** `urlSecretRegex` و `RedactURLSecrets()` اضافه شدند؛
  `subscription_daemon.go` آدرس اشتراک را **همیشه** ماسک می‌کند.
- **M20 (باگ):** `Profile.EffectiveCategory()` اضافه شد تا پروفایل‌های قدیمی
  (با `Category == ""`) از تست تأخیر جا نمانند.
- **M9 (باگ):** فیلد جداگانهٔ `probe config.Protocol` اضافه شد تا هدف WARP با پروب
  WireGuard/UDP سنجیده شود.
- **M11:** هوک `frontend/src/lib/useTimers.ts` اضافه شد؛ ۱۳ تایمر بدون cleanup ایمن شدند.
- **L1:** خطاهای رجیستری در `sysproxy_windows.go` دیگر دور ریخته نمی‌شوند
  (`writeProxySettings` خطای واقعی برمی‌گرداند؛ `clearSystemProxyPlatform` اولین خطا را جمع می‌کند).
- **L3/L7:** تابع تکراری `isRFC1918_172` حذف و `core.IsPrivate172` جایگزین شد؛
  `json.Unmarshal` بی‌معنا روی محتوای INI در `GenerateWarpOnWarp` حذف شد.
- **L5:** ۹ فلگ `ENABLE_DEPRECATED_*` مستند شدند و `singboxSchemaHint` راهنمای خطای اسکیما می‌دهد.
- **L9:** `.freebuff/`، `.git/cursor/` و `.git/opencode/` حذف شدند؛ `.gitignore` برای
  `*.bak`/`*.orig`/`*.rej` سخت‌سازی شد.
- **L10:** سه فایل تست رگرسیون اضافه شد (`internal/core`، `internal/config`، `internal/parser`).
- **L11:** CI حالا `go vet ./...`، `go test -race ./...`، `npx tsc --noEmit` و
  `golangci-lint` (با `only-new-issues`) را در هر push اجرا می‌کند.

### یافتهٔ خارج از ممیزی (مهم)

هنگام بازبینی ساختاری، **هشت محل کد مخدوش/تکرارشده در `internal/parser/parser.go`** کشف شد
که باعث می‌شد این پکیج اصلاً کامپایل نشود (بلوک‌های تکراری، `return` تکراری، فیلد تکراری در
struct literal، و `if`های بسته‌نشده). همه ترمیم شدند. جزئیات در
`docs/REMEDIATION_SUMMARY_FA.md`.

## ۲) محدودیت‌های اعتبارسنجی (مهم)

- در محیط اجرای این ممیزی، **زنجیره ابزار Go نصب نبود**؛ بنابراین `go build`، `go vet` و `go test`
  اجرا نشده‌اند. تمام تغییرات Go بر پایه بازبینی ایستا و تطبیق دقیق متن اعمال شده‌اند.
  **پیش از انتشار، یک‌بار روی ماشینی با Go 1.22+ اجرا شود:**
  ```bash
  go build ./... && go vet ./... && go test -race ./...
  ```
- تغییرات فرانت‌اند با کامپایلر TypeScript (`tsc --noEmit`) اعتبارسنجی شده‌اند (exit code = 0).
- هیچ‌کدام از موارد بالا به‌معنای تأیید عملکرد در زمان اجرا (runtime) نیست؛ تست دستی اتصال
  روی Windows همچنان لازم است.

## ۳) کارهای باقی‌مانده (جزئی)

- **M3:** باقی‌ماندهٔ `_ = ...` در Go عمدی است (بستن فایل/سوکت که خطای آن قابل‌اقدام نیست)
  و دیگر بلوک `if err != nil {}` خالی در Go وجود ندارد. در فرانت‌اند، هر ۷ `catch {}` خالی
  با توضیح «نادیده‌گرفتن آگاهانه» مستند شد و `Logs.loadLogs` دیگر خطا را بی‌صدا نمی‌خورد.
- **L5:** حذف وابستگی به فلگ‌های `ENABLE_DEPRECATED_*` پس از ارتقای هستهٔ sing-box
  (نیازمند بیلد تازه؛ فراتر از دامنهٔ بازبینی کد).
- **M14:** ~~`dist/` فقط بیلدهای 1.3.2 و 1.3.3 را دارد.~~ **حل شد (۱۴۰۵/۰۷/۲۹):**
  `frontend/dist` تازه بیلد شده و هیچ فایلی در `frontend/src` جدیدتر از آن نیست؛ و چون
  `dist/` در `.gitignore` است، هر بیلد (Local یا CI) آن را بازتولید می‌کند.
- **M1 (تکمیلی):** `noUnusedLocals` و `noUnusedParameters` **روشن شدند** و بدون خطا پاس می‌شوند
  (بخش ۴). کاهش بیشترِ `any`های صریح (باقی‌مانده عمدتاً پل runtime تزریق‌شدهٔ Wails و
  `catch (err)`های دینامیک‌اند) در صورت نیاز ادامه‌پذیر است.

## ۴) کارهای انجام‌شده در ادامهٔ رفع (جلسهٔ تکمیلی)

- **M1 (روشن‌کردن فلگ‌ها):** ۸۰+ import بلااستفاده و ۲۳ محلی/تابع بلااستفاده در ۳۶ فایل
  حذف شد؛ `noUnusedLocals`/`noUnusedParameters` در `tsconfig.json` فعال شدند و `tsc --noEmit`
  با exit code = ۰ پاس می‌شود. چند پنل/کنترل گم‌شده هم بازگردانده شد (ورودی جست‌وجوی لاگ‌ها،
  فیلد رمز رمزنگاری بک‌آپ ابری، چک‌باکس «جایگزینی هنگام بازیابی»، نمایش toast در صفحهٔ WireGuard).
  نوع `DNSScanResult` به `types` اضافه شد و `any`های متناظر با آن حذف شدند.
- **M12 (تعمیق a11y):** هوک `lib/useModalA11y.ts` (تلهٔ فوکوس Tab/Shift+Tab، بستن با Escape،
  انتقال فوکوس به مودال و بازگرداندن آن پس از بستن) روی **۲۷ مودال** اعمال شد؛ همهٔ آن‌ها
  `role="dialog"` و `aria-modal="true"` گرفتند. `components/AriaLive.tsx` + `lib/announcer.ts`
  به‌عنوان ناحیهٔ زندهٔ ARIA (polite/assertive) اضافه و در ریشهٔ اپ mount شد؛ رویدادهای
  اتصال/قطع/خطا و toast صفحهٔ WireGuard به آن اعلام می‌شوند.
- **L10 (تست مسیرهای بحرانی):** سه فایل تست رگرسیون اضافه شد:
  `internal/core/killswitch_test.go` (اعتبارسنجی آرگومان — باگ تاریخی «n/r/t»، و resolver آی‌پی سرور)،
  `internal/protocols/openvpn_test.go` (نرمال‌سازی کانفیگ: حذف `dev-node`، حذف
  `explicit-exit-notify` روی TCP، توازن کوتیشن)، و `internal/sysproxy/sysproxy_windows_test.go`
  (ساختار بایت‌به‌بایت بلاب `DefaultConnectionSettings`).
- **پاک‌سازی:** فایل زائد `frontend/src/pages/Dashboard.tsx.bak_before_dedup` حذف شد.

## ۵) پاک‌سازی کامل `golangci-lint` (جلسهٔ تکمیلی)

CI گام `lint` (golangci-lint با `only-new-issues`) سرخ بود. پایلاین گزارش را به‌خاطر
`max-same-issues` پیش‌فرض **فهرست ناقص** می‌داد (هر پیام حداکثر ۳ بار)، پس ابتدا با
`--max-same-issues 0 --max-issues-per-linter 0` فهرست کامل گرفته شد و بعد همه رفع شدند.
نتیجهٔ نهایی: `golangci-lint run ./...` → **صفر ایراد** (قبلاً ۵۴ مورد گزارش‌شده).

- **errcheck (۴۲ مورد):** فراخوانی‌های بی‌نتیجهٔ `syscall.Call`/`windows.CloseHandle`/`FreeSid`/
  `dll.Release`/`procLocalFree`/`io.Copy`/`KillProcessTree`. قرارداد پذیرفته‌شده در پروژه:
  `_, _, _ = proc.Call(...)` برای فراخوانی عادی و `defer func() { _, _, _ = proc.Call(x) }()`
  برای defer (تا ارگومان‌ها همان لحظهٔ ورود به تابع ارزیابی شوند، نه زمان اجرای واقعی).
  در `openvpn.go`/`xray.go` به‌جای بلع خطا، همان الگوی `singbox.go` اعمال شد: خطا لاگ می‌شود،
  چون معنايش این است که پراسس زنده مانده و پورت را نگه داشته.
- **unused (۳ مورد):** حذف فیلد `lastError` از `AetherAdapter` (هیچ‌جا خوانده/نوشته نمی‌شد)،
  فیلد `statsCancel` از `PsiphonAdapter` (مشابه؛ در `singbox.go` و `xray.go` واقعاً استفاده
  می‌شود، پس فقط این دو مورد مرده بودند) و پراسیدهای بی‌استفادهٔ `postQuitMessage` در ترای.
- **gosimple/staticcheck (۳ مورد):**
  - `converter.go`: حذف nil-check اضافی روی `n.Outbound` (`len()` روی map نیل هم صفر است).
  - `scanner.go`: دو حلقهٔ کپی‌کردنی به `append(targets, slice...)` تبدیل شد.
  - `apps.go`: cutset سرویس `"\" \r\n"` — قبل از رفع داخل بک‌تیک بود، پس `\r\n` دو کاراکتر
    بک‌اسلش و حرف بود و نه CR/LF؛ یعنی trimming درست کار نمی‌کرد.
- **ineffassign (۵ مورد) و SA9003 (۱ مورد) در `parser.go`:** کد مردهٔ واقعی پیدا و حذف شد:
  - دو بلوک سایهٔ `hostHeader` (در مسیر VLESS و Trojan) که مقدارشان هیچ‌وقت خوانده نمی‌شد و
    بعداً در همان تابع دوباره از `q.Get("host")` محاسبه می‌شد.
  - یک جفت `fragLength`/`fragInterval` تکراری در بلوک اول FragmentEnabled که هیچ اثری نداشت
    (جفت مؤثر همان یکی پایین‌تر است).
  - شرط `AppRoutingMode` دو شاخهٔ خالی در تولید کانفیگ Xray؛ هر دو شاخه فقط کامنت داشتند و
    Xray هیچ روتینگ بر پایهٔ پروسه ندارد — به یک کامنت توضیحی تبدیل شد.

نکتهٔ فرآیند: چون `.gitattributes` سورس‌های Go را به LF پین می‌کند و کپی کار کر‌اف‌دار است،
اعتبارسنجی gofmt روی نسخهٔ LF‌نرمال‌شده در یک دایرکتوری موقت انجام شد، نه روی فایل اصلی.

## ۶) ریشهٔ واقعی سرخیِ CI و رفع آن (جلسهٔ تکمیلی ۲)

پس از پاس اول، لینت محلی سبز بود ولی CI هنوز سرخ ماند. لاگ دریافت شد و مشخص شد علت،
خودِ ایرادهای لینت نبوده:

- **گویٔ اصلی خرابی لینت:** `main.go` از `//go:embed all:frontend/dist` استفاده می‌کند و
  `dist/` در `.gitignore` است. گام `lint` هیچ‌وقت فرانت‌اند را بیلد نمی‌کرد، پس تنها خطای
  واقعی این بود: `invalid go:embed: build system did not supply embed configuration`.
  **رفع:** گام‌های Node + `npm ci && npm run build` به گام lint اضافه شد.
- **دومین کشف مهم:** `max-same-issues` پیش‌فرض golangci-lint هر پیام یکسان را حداکثر ۳ بار
  نشان می‌دهد. فهرست «۵۴ مورد» در واقع یک‌سوم بک‌لاگ بود؛ با `--max-same-issues 0` فهرست کامل
  (معادل ۷۹ مورد با نسخهٔ ۲.x که CI نصب می‌کرد) گرفته و همه رفع شد.
- **نسخهٔ لینتر پین شد:** CI از `version: latest` استفاده می‌کرد که مجموعهٔ لینترهای پیش‌فرض را
  با هر انتشار عوض می‌کرد. حالا `.golangci.yml` (نسخهٔ ۲) اضافه شده و اکشن به `v2.6.2` پین است.
  استایل‌هایی که پروژه نمی‌خواهد در همان فایل با دلیل مستند شده‌اند: ZWNJ فارسی در رشته‌ها
  (`ST1018`)، نام‌های سریال‌شدهٔ `http_port` (`ST1003` — تغییرشان یعنی تغییر فرمت سیم کابل)،
  تبدیل زنجیرهٔ `if/else` روی `runtime.GOOS` به `switch` (`QF1003`).
- **باگ واقعی در تست (نه در کد):** `go test -race ./...` روی هر دو ویندوز و لینوکس با
  `WARNING: DATA RACE` شکست می‌خورد. ریشه: بستار `got` در `corelog_test.go` از goroutine تست و
  همزمان از goroutine تایمرِ `time.AfterFunc` نوشته می‌شد. خودِ merger کاملاً mutex‌دار است و
  `a.log`/`Engine.AddLog` هم mutex دارند؛ پس این یک باگِ تست بود. رفع با نوع `logSink`
  قفل‌دار + تبدیل انتظار ثابتِ ۱۵۰ms به polling با سقف ۳ ثانیه (تاخیر تایمر در CI با `-race`
  مسئلهٔ جداگانه‌ای است).
- **mojibake در UI:** برچسب‌های منطقه در `detectLocation` دوباره کدگذاشته شده بودند
  (`ðŸ‡©ðŸ‡ª` به‌جای 🇩🇪) و یک کاراکتر کنترل U+0090 هم داشتند. همه به ایموجی درست برگردانده شدند.

نتیجه: `go build`, `go vet`, `gofmt` (روی کپی LF)، `go test ./internal/...` و
`golangci-lint run` (هم نسخهٔ ۱.۶۲.۲ و هم ۲.۶.۲، بدون سقف تعداد) همه **صفر ایراد**.

## ۷) نکتهٔ باقی‌ماندهٔ CI (محدودیت محیط، نه باگ کد)

گام `Test Go Packages` در CI `go test -race ./...` را اجرا می‌کند و `-race` به CGO نیاز دارد.روی این ماشین هیچ کامپایلر C نصب نیست، پس این گام به‌صورت محلی بازتولید نشد؛ به‌جای آن
بیش از ۴۰۰ فراخوانی بی‌نتیجه و یک رِیس واقعی را که لاگ CI نشان داد رفع کردیم؛ بررسی محلی
(`go test ./internal/... -count=1`) را روی همان بسته‌ها سبز نگه داشتیم.
