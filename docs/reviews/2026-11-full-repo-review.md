# گزارش کامل بررسی کد NexVPN (D:\App1)

دامنه: ~۴۰,۰۰۰ خط Go + ~۲۹,۶۰۰ خط TypeScript/React + زیرساخت بیلد. همه ایرادها با شماره فایل و خط، تأییدشده روی سورس فعلی.

---

## 🔴 بحرانی / با اولویت فوری

| # | محل | مشکل | راه‌حل |
|---|-----|------|--------|
| 1 | `internal/core/binary_downloader.go:458-512` | باینری هسته **قبل از تأیید SHA-256** روی دیسک نصب می‌شود؛ اگر بین استخراج و چک هش کرش/قطع برق بیفتد، هسته جایگزین‌شده (احتمالاً مخرب) برای همیشه می‌ماند | هش را قبل از هر استخراجی روی بافر دانلودمحاسبه کن؛ فقط بعد از تأیید extract کن |
| 2 | `internal/core/binary_downloader.go:692-756` | `UpdateGeoData` هیچ چک یکپارچگی ندارد و از هر میرر (شامل ghproxy) فقط با حداقل ۱۰۰KB قبول می‌کند + `os.Remove` قبل از `os.Rename` یعنی renameِ خراب، دیتابیس خوب قبلی را هم نابود می‌کند | هش upstream پین کن و اول rename، بعد delete در خطا |
| 3 | `internal/config/store.go:1519-1548` | بکاپ فول ادعا می‌کند «رمزها را حذف می‌کند» اما `RawURI` و `ConfigContent` حاوی UUID ویلس، پسورد تروجان/هیستریا، و **کلید خصوصی WireGuard** هستند | رمزها را از URI/کانفیگ هم پاک کن یا کل باندل را با AES-GCM و passphrase کاربر رمزنگاری کن (helper آماده در `secret_other.go` داری) |
| 4 | `app_insights.go:596-607` | نشت secret در باندل دیاگنوستیک: `redactProfilesForDiagnostics` رمزها را حذف می‌کند اما `SubscriptionURL` را نه — این URLها معمولاً توکن دارند | `SubscriptionURL` را هم redact کن |
| 5 | `internal/parser/parser.go:2488` | باگ فرمت‌استرینگ در TUIC: `***` هاردکد داخل Sprintf آرگومان‌ها را شیفت داده — host جای password و پورت رشته‌مانند → هر نود TUIC سابسکریپشن Clash خراب تولید می‌شود | ترتیب آرگومان‌ها را درست کن |
| 6 | `frontend/src/lib/qrcode.ts:12-39` | تولید QR همیشه رشته خالی برمی‌گرداند: شاخه `toStringSync` وجود ندارد (کد مرده) و مسیر fallback نتیجه async را sync می‌خواند | تابع را async کن و `await QRCode.toString(...)` برگردان |
| 7 | `app.go:92` | data race واقعی روی `app.ctx` در `OnSessionComplete` (بدون `ctxMu`) | از `app.appCtx()` استفاده کن |
| 8 | `bin/` در گیت | ۴ فایل موتور (openvpn.exe و DLLها) با وجود gitignore، tracked مانده‌اند و از طریق installer توزیع می‌شوند — دور زدن جریان دانلود CI + بورپ مخزن | `git rm -r --cached bin/` |

---

## 🟠 بالا

### بک‌اند Go
- **`app_tunnelbear.go:49-167`** — ریس بین close/assign ریلی: دو.Connect همزمان، یکی از ریلی‌ها را بدون Close لوک می‌کند (نشت listener + گوروتین). قفل را روی کل start→swap→close نگه دار.
- **`main.go:49-60`** — مسیر signal با مسیر shutdown عادی همزمان اجرا می‌شود (cleanup دوگانه فایروال/پروکسی)؛ ضمناً `ClearConnectJournal` را رد می‌کند → دفعه بعد پیغام دروغین «session ended unexpectedly». با `sync.Once` سری کن.
- **`app_insights.go:561-566`** — اگر `archive.Close()` خطا بدهد، فایل zip هیچ‌وقت بسته نمی‌شود؛ خطاهای داخلی `addFile` هم swallow می‌شوند → باندل ناقص با گزارش موفقیت.
- **تنظیمات (read-modify-write race)** — `app.go:296,361`، `app_watchdog.go:343/400`، `App.tsx:261-305`، `Settings.tsx:269-279`: همه با snapshot قدیمی save می‌کنند و تغییر همزمان‌ها پاک می‌شود. راه‌حل ریشه‌ای: `UpdateSettings(func(*Settings))` در استور + `UpdateSettings(partial)` بک‌اند برای فرانت.
- **`app_profile.go:425-537`** — `PingAllProfiles` mutual exclusion ندارد؛ sweep دوم cancel اول را برمی‌گرداند و نتایج stale بازنویسی می‌شوند.
- **`app.go:736`** — مقایسه نسخه آپدیت فقط `!=` است: downgrade یا `-beta` هم «آپدیت هست» می‌دهد. از `golang.org/x/mod/semver` استفاده کن.
- **`app_watchdog.go:70-95`** — خروجی netsh با codepage کنسول (غیر UTF-8) decode می‌شود → SSID فارسی/عربی/ایموجی mojibake و قانون SSID هیچ‌وقت match نمی‌شود. (بهتر: wlanapi.dll و رویداد محور)
- **`sysproxy/sysproxy_windows.go:207-225`** — تشخیص پروکسی کهنه با substring روی `127.0.0.1:108` → پروکسی شخصی کاربر روی پورت 1080 را هم پاک می‌کند. مقایسه با رشته دقیق app-owned + marker.
- **`config/store.go:1561-1567`** — import بکاپ، `Settings` خارجی را بدون normalize می‌پذیرد (می‌تواند `RemoteControllerTLS=false` یا webhook خارجی هم بیاورد).
- **`app_discovery.go:254-376`** — `ApplyCleanIPTo*` هیچ اعتبارسنجی IP ندارد؛ رشته خالی/خراب همه پروفایل‌های آن دسته را خراب می‌کند. `net.ParseIP` گارد کن. (خطای فرعی: `WarpEndpoint` با `port<=0` روی `ip:0` ست می‌شود)
- **`app_profile.go:560-566` / `app.go:559`** — پورت پروکسی لوکال 0 می‌ماند و باز `RealDelayPing(0,...)` صدا زده می‌شود؛ خطای واضح برگردان. ثابت‌های 10808/10809 در ۴ جا هاردکد شده — از settings بخوان.
- **`parser/parser.go:30-31`** — `rand.Int` بدون چک خطا → panic احتمالی روی شکست crypto/rand.
- **`app_extra.go:139-254` + `app_discovery.go:217-418`** — اسکنرها بعد از panic رویداد پایانی (`*_error`) را emit نمی‌کنند → اسپینر UI برای همیشه گیر می‌کند.
- **`app_extra.go:588-615`** — `PanicWipe` با این نام، هیچ purge واقعی انجام نمی‌دهد؛ یا نامش را عوض کن یا پاک‌سازی store/log را اضافه کن.
- **`core/binary_downloader.go:211-223,758`** — جستجوی باینری از CWD/PATH؛ اجرای اپ از پوشه نوشتنی (Downloads/USB) یعنی DLL/exe کاشته‌شده برنده می‌شود. در release فقط exec dir.
- **Go modules** — `x/crypto v0.33.0` از آستانه GO-2025-3487 (ssh) عقب است → v0.35.0+.

### فرانت‌اند
- **`Dashboard.tsx:1376+1562`** — دو QR modal همزمان باز می‌شوند (legacy + ProfileQRModal). یکی را حذف کن.
- **`Dashboard.tsx:354/1523/1534`** — `currentSettings` هرگز پر نمی‌شود؛ FragmentStudio/GamingMode همیشه با `{}` باز می‌شوند.
- **`Dashboard.tsx:1577-1590`** — دکمه disconnect ویجت شناور در واقع connect به پروفایل انتخابی است! هندلر واقعی disconnect بده.
- **`App.tsx:176-238`** — اگر runtime وایلز دیر تزریق شود، listenerها هرگز attach نمی‌شوند و fallback poll هم skip می‌شود (چون `hasWailsRuntime` یک بار موقع mount خوانده شده). اول `await waitForWails()`.
- **`V2RayPage.tsx:95/847-859`** — فیچر rename گروه مرده است (modal باز نمی‌شود) و callback خودش هم بدون try/catch است.
- **`DiscoveryHub.tsx:391-410`** — بعد از ذخیره sources سفارشی، fetch فوری با stale state قدیمی اجرا می‌شود.
- **`DiscoveryHub.tsx:459-523`** — خطای شروع اسکن هندل نشده → اسپینر برای همیشه true می‌ماند.
- **`Settings.tsx:449-479`** — import بکاپ بدون تأیید و overwrite=true؛ پسورد خالی هم validate نمی‌شود.
- **`Pages: V2RayPage:174-175 / WarpWireGuardPage:203-204`** — `setProfiles(updated)` بدون گارد Array.isArray → لیست یکجا سفید می‌شود.

---

## 🟡 متوسط (خلاصه)

**Go:**
- `app.go:481-492`: فیلور اضطراری Psiphon بدون cooldown/offline-gate → churn بی‌پایان آفلاین.
- `app_extra.go:588-615` گزینه‌های Psiphon از اولین پروفایل تصادفی برمی‌گردد نه پروفایل مقصد.
- `app_extra.go:186-209, app_profile.go:211-232`: خطاهای batch عملیات‌ها swallow می‌شوند (نتیجه ناقص = موفق).
- `app_extra.go:100`: `appCtx` بدون nil check + مسیر `file://` با بک‌اسلش ویندوزی (invalid URI).
- `app_extra.go:295-308`: لایسنس WARP+ قبل از موفقیت regenerate ذخیره می‌شود و normalized نیست.
- `app_network.go:77-217`: `RunProtocolRadar` تا ~۶۰ ثانیه sync روی UI thread، بدون context/cancel.
- `parser/parser.go:2819-2846`: lookup جغرافیایی روی **http** (نه https) — در تهدیدمدل DPI اپ، لو رفتن ISP/exit-IP.
- `core/notifier.go:34-44`: هر notification یک process بدون Wait → نشت هندل.
- `core/engine.go:1377-1387`: هر ۱۵۰ms یک PowerShell کامل برای wintun spawn می‌شود.
- `config/store.go:1460-1469`: Unlock بدون defer در `RecordConnectionSession` → deadlock روی panic.
- `sysproxy/apps.go:137`: پوشه ساختگی `"…"` در اسکن مسیر.
- `dnsscan/scanner.go`: تشخیص poison فقط prefix یک رنج + پاسخ DoH بدون validate هدر DNS؛ TXID ثابت.
- `app_extra.go:823-850` تست latency piston/psiphon و `app_tunnelbear.go:245-261`: dialهای ترتیبی با timeout بالا = UI سنگین.

**فرانت:**
- `lib/utils.ts:139-147`: تشخیص کشور با `includes` → پرچم غلط (croatia→ austریا و...). fallback substring را حذف کن.
- `App.tsx:531-537`: connect/disconnect بدون catch → unhandled rejection.
- `lib/api.ts:387`: در مسیر fallback، `EventsOff` همه subscriberهای آن event را می‌کُشد.
- `hooks/useSpeedTest.ts:137-179`: cleanup بدون started-ref → هر بار IPC اضافه و امکان کشتن تست دیگران.
- `ScreenQRScannerModal.tsx:147-167`: stream دوربین در مسیر خطا بسته نمی‌شود (چراغ دوربین روشن می‌ماند).
- `Logs.tsx:46-65`: دو مسیر آپدیت (poll + event) رقابت با هم → لاگ‌ها می‌پرند/جابجا؛ `key={idx}` هم مشکل‌ساز.
- `Settings.tsx:269-279`: rollback از snapshot قدیمی → toggle سریع دو سوئیچ، یکی را پاک می‌کند.
- `Pages`: راست‌به‌چپ IPC و timerهای بعد از unmount (Aether/Psiphon)، toast چهار نسخه با ۴ استایل/مکان، clipboard بدون catch با پیام «کپی شد» حتی در شکست، sort/search حساس به حروف بزرگ (OpenVPN/Warp)، i18n bypass با توست انگلیسی هاردکد.
- `main.css:30`: فونت‌ها با @import از CDN → لیک اینکه اپ استارت شده + FOUC در شبکه فیلتر. فونت را bundle کن.
- `Dashboard.tsx:438-449, PsiphonPage.tsx:339`: نمایش IP/پروتکل در حالت شکست، گمراه‌کننده.

**زیرساخت:**
- winres نسخه 1.4.0.0 vs version.go 1.4.1 vs package.json های مختلف → یک source of truth بساز.
- `build.ps1` به مسیر toolchain machine-specific وابسته است.
- بدون dependabot.yml و govulncheck؛ تست‌های vitest فرانت در CI اجرا نمی‌شوند.

---

## ✅ نکات مثبت (تأییدشده)
- DPAPI در `secret/` درست پیاده شده؛ رمزها موقع persist مهر و موقع load باز می‌شوند.
- هیچ command injectionای پیدا نشد (همه exec با argv؛ حتی دفاع injection در mtu ping).
- خواندن subscription با LimitReader، سقف unzip، مقایسه PIN با constant-time — همه درست.
- CI واقعی دارد: `go test -race` دو پلتفرم + golangci + vet + ۴۱ فایل تست.
- فرانت: هیچ sink خطرناک XSS نیست؛ همه IPC از wrapperهای typed می‌گذرد.

## 💡 پیشنهادهای توسعه (سود بیشترین به کمترین)
1. **`UpdateSettings(fn)` در استور + `UpdateSettings(partial)` در بایندینگ وایلز** — کلاس کامل باگ‌های lost-update را یکجا می‌بندد.
2. **`useCoreDownload` + `useProfileList` + `useToast` + `useScanJob` هوک‌های اشتراکی** — ~۴۰۰ خط کد تکراری در ۴ صفحه حذف می‌شود و باگ‌های stuck-spinner/ryw یکبار برای همیشه رفع می‌شوند.
3. **`scanManager` مشترک Go** برای الگوی mutex+cancel+emit+panic-safe در ۶ اسکنر.
4. **بکاپ v2 با passphrase (AES-GCM)** — مورد ۳ و ۱۰ را یکجا حل می‌کند.
5. **کد خطای تایپ‌دار از engine** به جای sniffing رشته (`AUTH_FAILED`) → پیام‌های فارسی بهتر و failover مطمئن‌تر.
6. **تست جامانس (contract/golden)** برای خروجی کانفیگ‌ها: اجرای `sing-box check` / `xray -test` روی هر شاخه builder در CI — باگ TUIC (بند ۵ بالا) با این تست catching می‌شد.
7. SSID با **wlanapi.dll** به جای netsh (UTF-16 درست + event-driven، بدون پولینگ ۴ ثانیه).
8. `govulncheck` + `npm audit` + اجرای vitest در build.yml + dependabot.
9. ذخیره تاریخچه reports رادار پروتکل (الان هر گزارش دور ریخته می‌شود).
10. در DiscoveryHub انقضای کش‌ها با timestamp (بج «alive» استیل تاکید).
