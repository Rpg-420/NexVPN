#Requires -Version 5.1
<#
    NexVPN-Report — standalone launch diagnostic.

    Why this is a separate tool and not a button inside the app
    -----------------------------------------------------------
    It has to work precisely when the app does not: a window that opens black,
    a launch that hangs before the loading screen, an app that closes itself, or
    a core that dies ten seconds after the UI already said "connected". It
    therefore depends on nothing from NexVPN — only Windows PowerShell 5.1 and
    the startup.log the Go backend writes as the launch progresses.

    What it does
    ------------
      1. Finds the NexVPN data directory.
      2. Reads startup.log, isolates the most recent launch, and says in plain
         Persian how far that launch got and what stopped it.
      3. Lists every failure it can see in that launch.
      4. Collects machine state that matters for the known failure modes —
         a ghost Wintun adapter, leftover core processes, missing admin rights.
      5. Opens the verdict in Notepad and drops a single .zip on the Desktop.

    Encoding note for maintainers
    -----------------------------
    This file MUST keep its UTF-8 byte-order mark. Windows PowerShell 5.1 reads
    a .ps1 without a BOM as ANSI, and every Persian string in here becomes
    mojibake. If you regenerate this file, re-add the BOM.

    Privacy note
    ------------
    The archive deliberately does NOT include config.json or history.json. Both
    contain subscription URLs with embedded tokens, and a diagnostic bundle is
    exactly the kind of file people email to a stranger. The launch diary is
    written through the app's own redaction layer, so it is safe to share.
#>
[CmdletBinding()]
param(
    # Skip creating the .zip (useful when only the verdict is wanted).
    [switch]$NoZip,
    # Print the verdict to the console instead of opening Notepad.
    [switch]$NoNotepad
)

$ErrorActionPreference = 'Continue'
$ProgressPreference = 'SilentlyContinue'

# ---------------------------------------------------------------------------
# Event vocabulary. Kept in sync with internal/diary and internal/core/engine.go.
# ---------------------------------------------------------------------------

$StepOrder = @(
    @{ Key = 'launch/main-enter';             Label = 'اجرای فایل برنامه' }
    @{ Key = 'launch/app-constructed';        Label = 'ساخت اولیهٔ اجزای برنامه' }
    @{ Key = 'launch/stale-cleanup-done';     Label = 'پاک‌سازی وضعیت قبلی سیستم' }
    @{ Key = 'launch/tray-ready';             Label = 'ساخت آیکون سینی سیستم' }
    @{ Key = 'launch/store-loaded';           Label = 'خواندن تنظیمات و پروفایل‌ها' }
    @{ Key = 'launch/daemons-started';        Label = 'راه‌اندازی سرویس‌های پس‌زمینه' }
    @{ Key = 'launch/ready';                  Label = 'آماده‌شدن پنجرهٔ برنامه' }
    @{ Key = 'connect/tunnelbear-prepare';    Label = 'آماده‌سازی سرور TunnelBear و رلهٔ محلی' }
    @{ Key = 'connect/begin';                 Label = 'شروع اتصال' }
    @{ Key = 'connect/port-owner-killed';     Label = 'آزادسازی پورت پروکسی از یک هستهٔ باقی‌مانده' }
    @{ Key = 'connect/port-still-busy';       Label = 'آزاد نشدن پورت پروکسی' }
    @{ Key = 'connect/proxy-port-busy';       Label = 'اشغال بودن پورت پروکسی هنگام شروع اتصال' }
    @{ Key = 'connect/tun-prepare';           Label = 'پاک‌سازی آداپتور TUN پیش از اجرای هسته' }
    @{ Key = 'connect/tun-ghost-avoided';     Label = 'دور زدن آداپتور TUN شبح‌شده' }
    @{ Key = 'connect/tun-adapter-preexisting'; Label = 'وجود آداپتور TUN از قبل' }
    @{ Key = 'connect/tun-needs-admin';       Label = 'نیاز حالت TUN به دسترسی Administrator' }
    @{ Key = 'connect/psiphon-core-missing';  Label = 'نبود هستهٔ سایفون و بازگشت به تونل اضطراری' }
    @{ Key = 'connect/engine-select';         Label = 'انتخاب موتور مناسب' }
    @{ Key = 'connect/spawn';                 Label = 'اجرای هستهٔ VPN' }
    @{ Key = 'connect/tun-ready';             Label = 'بالا آمدن آداپتور TUN' }
    @{ Key = 'connect/proxy-core-ready';      Label = 'آماده‌شدن هستهٔ پروکسی' }
    @{ Key = 'connect/core-exited';           Label = 'از کار افتادن هستهٔ VPN' }
    @{ Key = 'connect/proxy-port-timeout';    Label = 'باز نشدن پورت پروکسی' }
    @{ Key = 'connect/tun-adapter-failed';    Label = 'شکست آداپتور TUN به‌عنوان علت واقعی' }
    @{ Key = 'connect/verify-ok';             Label = 'تأیید عبور ترافیک از تونل' }
    @{ Key = 'connect/connected';             Label = 'اتصال کامل' }
    @{ Key = 'connect/failover-begin';        Label = 'شروع تعویض خودکار سرور' }
    @{ Key = 'connect/failover-probe-ok';     Label = 'یافتن سرور جانشین سالم' }
    @{ Key = 'connect/failover-blind-attempt'; Label = 'تلاش کورکورانه روی سرور جانشین' }
    @{ Key = 'connect/failover-none-reachable'; Label = 'پاسخ‌ندادن هیچ سرور جانشین' }
    @{ Key = 'connect/failover-skip';         Label = 'رد شدن از تعویض خودکار سرور' }
    @{ Key = 'disconnect/begin';              Label = 'شروع قطع اتصال' }
    @{ Key = 'disconnect/done';               Label = 'پایان قطع اتصال' }
)

$Verdicts = @{
    'launch/main-enter'                = 'برنامه اجرا شد، ولی حتی به مرحلهٔ ساخت اجزای داخلی نرسید. یعنی هنگام ساختن خودِ برنامه متوقف شده است.'
    'launch/app-constructed'           = 'اجزای داخلی برنامه ساخته شد، ولی پنجره هرگز باز نشد. مشکل در راه‌اندازی رابط گرافیکی یا WebView2 است.'
    'launch/stale-cleanup-done'        = 'پاک‌سازی اولیه انجام شد، ولی برنامه پیش از ساخت آیکون سینی متوقف شد.'
    'launch/store-load-failed'         = 'خواندن فایل تنظیمات شکست خورد. فایل config.json احتمالاً خراب یا غیرقابل‌خواندن است.'
    'launch/store-loaded'              = 'تنظیمات و پروفایل‌ها خوانده شد، ولی سرویس‌های پس‌زمینه بالا نیامدند.'
    'launch/daemons-started'           = 'سرویس‌های پس‌زمینه بالا آمدند، ولی پنجره هرگز به حالت «آماده» نرسید.'
    'launch/ready'                     = 'برنامه با موفقیت بالا آمد. اگر مشکل ادامه دارد، مربوط به مرحلهٔ اتصال است نه اجرا.'
    'launch/auto-connect-queued'       = 'برنامه بالا آمد و اتصال خودکار در صف قرار گرفت، ولی اتصال شروع نشد.'
    'launch/signal-received'           = 'برنامه سیگنال خروج گرفت (بسته شدن یا kill شدن فرآیند).'
    'launch/wails-exited'              = 'حلقهٔ اصلی برنامه به‌طور عادی تمام شد.'
    'launch/wails-exited-with-error'   = 'حلقهٔ اصلی برنامه با خطا تمام شد. جزئیات در خط زمانی پایین آمده است.'
    'connect/begin'                    = 'اتصال شروع شد، ولی هیچ موتوری انتخاب نشد.'
    'connect/engine-select-failed'     = 'انتخاب موتور شکست خورد — یعنی هیچ هستهٔ مناسبی برای این پروفایل پیدا یا آماده نشد.'
    'connect/engine-select'            = 'موتور انتخاب شد، ولی اجرا نشد.'
    'connect/phase'                    = 'زمان‌سنجی یکی از مرحله‌های اتصال. هر خط می‌گوید کدام مرحله (step) چقدر طول کشیده (took) — اگر فاصلهٔ بین «شروع اتصال» و «انتخاب موتور» زیاد است، همین خط‌ها نشان می‌دهند کدام مرحله کند است.'
    'connect/spawn'                    = 'هستهٔ VPN اجرا شد، ولی هرگز به حالت آماده نرسید.'
    'connect/tun-ready'                = 'آداپتور TUN با موفقیت بالا آمد.'
    'connect/tun-timeout'              = 'آداپتور TUN در زمان مقرر بالا نیامد. این همان نقص شناخته‌شدهٔ درایور Wintun است که معمولاً با حذف آداپتور باقی‌مانده حل می‌شود.'
    'connect/core-exited-early'        = 'هستهٔ VPN بلافاصله بعد از اجرا از کار افتاد. متن خطای دقیق هسته در خط زمانی پایین آمده است.'
    'connect/core-exited'              = 'هستهٔ VPN در میانهٔ کار از کار افتاد. اگر بعد از این خط، تعویض خودکار سرور نیامده باشد، اتصال بی‌صدا رها شده است.'
    'connect/proxy-port-timeout'       = 'هستهٔ VPN اجرا بود ولی پورت پروکسی محلی را باز نکرد. یعنی هسته زنده ماند اما کار نکرد.'
    'connect/proxy-core-ready'         = 'هستهٔ پروکسی آماده شد.'
    'connect/fallback'                 = 'حالت TUN شکست خورد و برنامه به‌طور خودکار به حالت System Proxy برگشت.'
    'connect/fallback-ok'              = 'برگشت خودکار به حالت System Proxy موفق بود.'
    'connect/verify-ok'                = 'ترافیک با موفقیت از تونل عبور کرد.'
    'connect/verify-fail'              = 'تأیید تونل شکست خورد — سرور از داخل تونل هیچ پاسخی نداد. یعنی پروفایل مرده، مسدود، یا اشتباه تنظیم شده است.'
    'connect/connected'                = 'اتصال به‌طور کامل برقرار شد.'
    'connect/error'                    = 'اتصال با خطا متوقف شد.'
    'connect/failover-begin'           = 'تعویض خودکار سرور شروع شد.'
    'connect/failover-probe-ok'        = 'سرور جانشین سالم پیدا شد و برنامه به آن سوئیچ کرد.'
    'connect/failover-blind-attempt'   = 'هیچ سرور جانشینی به پروب پاسخ نداد؛ برنامه یک تلاش کورکورانه روی پروفایلی کرد که آدرسش قابل بررسی نبود.'
    'connect/failover-none-reachable'  = 'هیچ سرور جانشینی پاسخ نداد، پس برنامه عمداً متصل نشد. این تقریباً همیشه یعنی مشکل از شبکه یا ISP است، نه از یک سرور خاص.'
    'connect/failover-skip'            = 'تعویض خودکار سرور اجرا شد ولی رد شد. دلیل دقیق در خط زمانی پایین آمده است.'
    'connect/tunnelbear-prepare'       = 'پروفایل TunnelBear آماده شد: نشانی سرورها پیدا شد، رلهٔ محلی روی loopback بالا آمد و حساب ذخیره‌شده به آن تزریق شد. خط‌های بعدی نشان می‌دهند خودِ تونل بالا آمد یا نه.'
    'connect/tunnelbear-prepare-failed' = 'آماده‌سازی پروفایل TunnelBear شکست خورد، پس اتصال حتی شروع نشد. علت معمولاً یکی از این‌هاست: حسابی ذخیره نشده، رمز ذخیره‌شده با حساب ویندوز دیگری رمزگذاری شده، یا نام سرور حل نشد.'
    'connect/port-owner-killed'        = 'پورت پروکسی محلی (۱۰۸۰۸) در دست یک هستهٔ باقی‌مانده بود و برنامه پیش از ادامه آن را بست. این حالت یعنی اجرای قبلی به‌طور تمیز بسته نشده بود.'
    'connect/port-still-busy'          = 'پورت پروکسی محلی آزاد نشد. اگر عامل آن یک برنامهٔ دیگر باشد، اتصال هرگز شروع نمی‌شود؛ برنامه‌های VPN دیگر را ببندید.'
    'connect/proxy-port-busy'          = 'اتصال پیش از شروع متوقف شد چون پورت ۱۰۸۰۸ آزاد نبود. علت معمولاً یک هستهٔ VPN باقی‌مانده از اجرای قبلی یا یک کلاینت VPN دیگر است.'
    'connect/tun-prepare'              = 'برنامه آداپتور TUN خودش را پیش از اجرای هسته پاک کرد تا هسته مجبور نشود از یک اینترفیس باقی‌مانده استفاده کند.'
    'connect/tun-ghost-avoided'        = 'آداپتور TUN پیش‌فرض به شکل «شبح» روی سیستم مانده بود — درایور Wintun هنوز نامش را می‌شناسد ولی ویندوز هیچ آداپتوری با آن نام ندارد — پس برنامه نام دیگری برای این اتصال انتخاب کرد. این وضعیت تا وقتی آداپتور شبح در رجیستری باقی است ادامه دارد.'
    'connect/tun-adapter-preexisting'  = 'آداپتور TUN از قبل روی سیستم وجود داشت. همین موضوع باعث می‌شد هستهٔ VPN «آماده» به نظر برسد در حالی که در واقع از کار می‌افتاد؛ خط‌های بعدی نشان می‌دهند کدام حالت رخ داده است.'
    'connect/psiphon-core-missing'     = 'فایل psiphon.exe روی سیستم پیدا نشد، پس درخواست سایفون به تونل اضطراری sing-box منتقل شد. برای استفاده از خود سایفون باید psiphon.exe در پوشهٔ bin کنار برنامه باشد.'
    'connect/tun-needs-admin'          = 'حالت TUN درخواست شده بود ولی برنامه با دسترسی Administrator اجرا نشده بود. ویندوز ساخت و پیکربندی آداپتور شبکه (Wintun) را فقط به برنامهٔ ارتقایافته اجازه می‌دهد، پس هسته در همان لحظهٔ ساخت آداپتور با خطای «Access is denied» از کار می‌افتد — خطایی که هیچ ربطی به سرور ندارد. برای اینکه کاربر بدون اتصال نماند، برنامه به‌طور خودکار به حالت System Proxy برگشت. راه‌حل: اجرای NexVPN با «Run as administrator».'
    'connect/tun-adapter-failed'       = 'هستهٔ VPN به‌خاطر آداپتور TUN از کار افتاد، نه به‌خاطر سرور. چون این نقص محلی است و روی همهٔ سرورها یکسان تکرار می‌شود، برنامه از تعویض خودکار سرور صرف‌نظر کرد تا وقت و سهمیهٔ تلاش هدر نرود و علت واقعی یک بار گفته شود.'
    'disconnect/begin'                 = 'قطع اتصال شروع شد.'
    'disconnect/done'                  = 'اتصال به‌طور تمیز قطع شد.'
}

# Events worth pulling out as "problems found", in the order they appear.
$ProblemEvents = @(
    'store-load-failed', 'engine-select-failed', 'core-exited-early', 'core-exited',
    'proxy-port-timeout', 'proxy-port-busy', 'port-still-busy', 'tun-timeout',
    'tun-adapter-preexisting', 'tun-ghost-avoided', 'tun-needs-admin', 'tun-adapter-failed',
    'psiphon-core-missing',
    'verify-fail', 'fallback',
    'failover-none-reachable', 'tunnelbear-prepare-failed', 'error', 'wails-exited-with-error'
)

$PersianProblemLabels = @{
    'store-load-failed'        = 'خواندن تنظیمات شکست خورد'
    'engine-select-failed'     = 'انتخاب موتور شکست خورد'
    'core-exited-early'        = 'هستهٔ VPN بلافاصله از کار افتاد'
    'core-exited'              = 'هستهٔ VPN در میانهٔ کار از کار افتاد'
    'proxy-port-timeout'       = 'پورت پروکسی محلی باز نشد'
    'proxy-port-busy'          = 'پورت پروکسی محلی هنگام شروع اتصال اشغال بود'
    'port-still-busy'          = 'پورت پروکسی محلی آزاد نشد'
    'tun-timeout'              = 'آداپتور TUN در زمان مقرر بالا نیامد'
    'tun-adapter-preexisting'  = 'آداپتور TUN از قبل روی سیستم وجود داشت'
    'tun-ghost-avoided'        = 'آداپتور TUN پیش‌فرض شبح‌شده بود و دور زده شد'
    'tun-needs-admin'          = 'حالت TUN بدون دسترسی Administrator درخواست شده بود'
    'tun-adapter-failed'       = 'هسته به‌خاطر آداپتور TUN از کار افتاد، نه به‌خاطر سرور'
    'psiphon-core-missing'     = 'هستهٔ سایفون روی سیستم نبود و به‌جای آن تونل اضطراری sing-box اجرا شد'
    'verify-fail'              = 'تأیید عبور ترافیک از تونل شکست خورد'
    'fallback'                 = 'حالت TUN شکست خورد و به System Proxy برگشت'
    'failover-none-reachable'  = 'هیچ سرور جانشینی پاسخ نداد'
    'tunnelbear-prepare-failed' = 'آماده‌سازی پروفایل TunnelBear شکست خورد'
    'error'                    = 'خطا در جریان اتصال'
    'wails-exited-with-error'  = 'حلقهٔ اصلی برنامه با خطا تمام شد'
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

function Get-NexVPNDirs {
    $candidates = New-Object System.Collections.ArrayList
    $roaming = $env:APPDATA
    if (-not $roaming) { $roaming = [Environment]::GetFolderPath('ApplicationData') }
    $local = $env:LOCALAPPDATA
    if (-not $local) { $local = [Environment]::GetFolderPath('LocalApplicationData') }

    if ($roaming) { [void]$candidates.Add((Join-Path $roaming 'NexVPN')) }
    if ($local)   { [void]$candidates.Add((Join-Path $local   'NexVPN')) }
    return $candidates
}

function Read-DiaryEvents {
    param([string]$Path)

    $events = New-Object System.Collections.ArrayList
    if (-not (Test-Path -LiteralPath $Path)) { return $events }

    $lines = @()
    try {
        $lines = [System.IO.File]::ReadAllLines($Path, [System.Text.Encoding]::UTF8)
    } catch {
        return $events
    }

    foreach ($line in $lines) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $parts = $line -split "`t"
        if ($parts.Count -lt 3) { continue }

        $data = @{}
        for ($i = 3; $i -lt $parts.Count; $i++) {
            $pair = $parts[$i]
            $eq = $pair.IndexOf('=')
            if ($eq -gt 0) {
                $data[$pair.Substring(0, $eq)] = $pair.Substring($eq + 1)
            }
        }

        $null = $events.Add([PSCustomObject]@{
            Time     = $parts[0]
            Category = $parts[1]
            Event    = $parts[2]
            Data     = $data
            Key      = $parts[1] + '/' + $parts[2]
        })
    }
    return $events
}

function Format-EventTime {
    param([string]$Stamp)
    # RFC3339Nano -> a short local time. Falls back to the raw stamp.
    try {
        return ([DateTimeOffset]::Parse($Stamp)).LocalDateTime.ToString('HH:mm:ss.fff')
    } catch {
        return $Stamp
    }
}

function Get-SystemFindings {
    $out = New-Object System.Collections.ArrayList

    try {
        $os = Get-CimInstance -ClassName Win32_OperatingSystem -ErrorAction Stop
        [void]$out.Add('سیستم‌عامل: ' + $os.Caption + ' (' + $os.Version + ')')
    } catch {
        [void]$out.Add('سیستم‌عامل: نامشخص (Win32_OperatingSystem خوانده نشد)')
    }

    [void]$out.Add('معماری پردازنده: ' + $env:PROCESSOR_ARCHITECTURE)

    $isAdmin = $false
    try {
        $identity  = [Security.Principal.WindowsIdentity]::GetCurrent()
        $principal = New-Object Security.Principal.WindowsPrincipal($identity)
        $isAdmin   = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    } catch { }
    if ($isAdmin) {
        [void]$out.Add('دسترسی Administrator: دارد')
    } else {
        [void]$out.Add('دسترسی Administrator: ندارد — حالت TUN و تغییرات DNS بدون این دسترسی کار نمی‌کنند.')
    }

    # A NexVPN process still alive means the report was run while the app is up.
    $appProc = Get-Process -Name 'NexVPN' -ErrorAction SilentlyContinue
    if ($appProc) {
        [void]$out.Add('وضعیت برنامه: همین حالا در حال اجراست (PID: ' + ($appProc.Id -join ', ') + ')')
    } else {
        [void]$out.Add('وضعیت برنامه: اجرا نمی‌شود')
    }

    # Leftover cores hold ports 10808/10809 and block the next connection.
    $coreNames = @('sing-box', 'xray', 'openvpn', 'wintun')
    $alive = New-Object System.Collections.ArrayList
    foreach ($n in $coreNames) {
        $p = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($p) { [void]$alive.Add($n + ' (' + ($p.Id -join ',') + ')') }
    }
    if ($alive.Count -gt 0) {
        [void]$out.Add('هسته‌های باقی‌مانده: ' + ($alive -join ' ، ') + ' — این‌ها پورت‌ها را اشغال می‌کنند و باید بسته شوند.')
    } else {
        [void]$out.Add('هسته‌های باقی‌مانده: ندارد')
    }

    # The ghost-adapter condition: the name is registered in the driver but the
    # device node is gone, which is why netsh cannot see or re-enable it.
    try {
        $adapters = Get-NetAdapter -IncludeHidden -ErrorAction Stop |
            Where-Object { $_.Name -like 'NexVPN*' -or $_.InterfaceDescription -like '*NexVPN*' }
        if ($adapters) {
            foreach ($a in $adapters) {
                [void]$out.Add('آداپتور NexVPN: نام=' + $a.Name + ' وضعیت=' + $a.Status + ' توضیح=' + $a.InterfaceDescription)
            }
        } else {
            [void]$out.Add('آداپتور NexVPN: وجود ندارد (پاک است)')
        }
    } catch {
        [void]$out.Add('آداپتور NexVPN: بررسی نشد (Get-NetAdapter در دسترس نبود)')
    }

    try {
        $pnp = Get-PnpDevice -Class Net -ErrorAction Stop |
            Where-Object { $_.FriendlyName -like '*NexVPN*' }
        if ($pnp) {
            foreach ($d in $pnp) {
                [void]$out.Add('دستگاه PnP: ' + $d.FriendlyName + ' وضعیت=' + $d.Status)
            }
        } else {
            [void]$out.Add('دستگاه PnP: موردی با نام NexVPN ثبت نشده')
        }
    } catch {
        [void]$out.Add('دستگاه PnP: بررسی نشد (Get-PnpDevice در دسترس نبود)')
    }

    return $out
}

# ---------------------------------------------------------------------------
# 1. Locate the diary
# ---------------------------------------------------------------------------

$dataDir = $null
$diaryPath = $null
foreach ($d in Get-NexVPNDirs) {
    if (Test-Path -LiteralPath $d) {
        $candidate = Join-Path $d 'startup.log'
        if (Test-Path -LiteralPath $candidate) {
            $dataDir = $d
            $diaryPath = $candidate
            break
        }
        if (-not $dataDir) { $dataDir = $d }
    }
}
if (-not $dataDir) {
    $dirs = Get-NexVPNDirs
    $dataDir = $dirs[0]
}
if (-not $diaryPath) {
    $diaryPath = Join-Path $dataDir 'startup.log'
}

$events = Read-DiaryEvents -Path $diaryPath

# ---------------------------------------------------------------------------
# 2. Isolate the most recent launch
# ---------------------------------------------------------------------------

$lastBegin = -1
for ($i = 0; $i -lt $events.Count; $i++) {
    if ($events[$i].Key -eq 'launch/begin') { $lastBegin = $i }
}

$launch = @()
if ($lastBegin -ge 0) {
    $launch = $events[$lastBegin..($events.Count - 1)]
}

# ---------------------------------------------------------------------------
# 3. Build the verdict
# ---------------------------------------------------------------------------

$lines = New-Object System.Collections.ArrayList

[void]$lines.Add('گزارش راه‌اندازی NexVPN')
[void]$lines.Add('ساخته‌شده در: ' + (Get-Date).ToString('yyyy-MM-dd HH:mm:ss'))
[void]$lines.Add('')
[void]$lines.Add('پوشهٔ داده: ' + $dataDir)
[void]$lines.Add('فایل دفترچه: ' + $diaryPath)
[void]$lines.Add('')

$summary = ''

if ($events.Count -eq 0) {
    $summary = 'فایل دفترچهٔ راه‌اندازی پیدا نشد یا خالی است.'
    [void]$lines.Add('=== خلاصه ===')
    [void]$lines.Add($summary)
    [void]$lines.Add('')
    [void]$lines.Add('معنی این وضعیت:')
    [void]$lines.Add('  - یا برنامه از نسخه‌ای قدیمی‌تر از ثبت دفترچه استفاده می‌کند،')
    [void]$lines.Add('  - یا هرگز روی این حساب کاربری اجرا نشده است،')
    [void]$lines.Add('  - یا پوشهٔ داده جای دیگری است.')
    [void]$lines.Add('')
} else {
    # How far did it get? Highest step index reached in this launch.
    $reached = -1
    $reachedEvent = $null
    foreach ($e in $launch) {
        for ($s = 0; $s -lt $StepOrder.Count; $s++) {
            if ($StepOrder[$s].Key -eq $e.Key -and $s -gt $reached) {
                $reached = $s
                $reachedEvent = $e
            }
        }
    }

    # The last event we have a plain-Persian sentence for.
    $lastKnown = $null
    foreach ($e in $launch) {
        if ($Verdicts.ContainsKey($e.Key)) { $lastKnown = $e }
    }

    # Problems found. Collected before the summary because the headline should
    # name the first real failure, not merely the last event in the file — a
    # launch that failed and then recovered via fallback would otherwise be
    # summarised as if nothing had gone wrong.
    $problems = New-Object System.Collections.ArrayList
    foreach ($e in $launch) {
        if ($ProblemEvents -contains $e.Event) { [void]$problems.Add($e) }
    }

    [void]$lines.Add('=== خلاصه ===')
    if ($reached -ge 0) {
        $summary = 'آخرین اجرا تا مرحلهٔ «' + $StepOrder[$reached].Label + '» پیش رفت.'
    } else {
        $summary = 'آخرین اجرا هیچ مرحلهٔ شناخته‌شده‌ای را ثبت نکرد.'
    }
    if ($problems.Count -gt 0) {
        $firstLabel = $PersianProblemLabels[$problems[0].Event]
        if (-not $firstLabel) { $firstLabel = $problems[0].Event }
        $summary = $summary + ' سپس شکست خورد — اولین مشکل: ' + $firstLabel + ' در ساعت ' + (Format-EventTime $problems[0].Time) + '.'
    }
    [void]$lines.Add($summary)

    if ($lastKnown) {
        [void]$lines.Add('')
        [void]$lines.Add('آخرین رویداد ثبت‌شده: ' + $Verdicts[$lastKnown.Key])
    }
    [void]$lines.Add('')

    [void]$lines.Add('=== مشکلات یافت‌شده در این اجرا ===')
    if ($problems.Count -eq 0) {
        [void]$lines.Add('موردی ثبت نشده است.')
    } else {
        $n = 0
        foreach ($p in $problems) {
            $n++
            $label = $PersianProblemLabels[$p.Event]
            if (-not $label) { $label = $p.Event }
            $detail = ''
            if ($p.Data.ContainsKey('err'))     { $detail = ' — ' + $p.Data['err'] }
            elseif ($p.Data.ContainsKey('reason')) { $detail = ' — ' + $p.Data['reason'] }
            [void]$lines.Add('  ' + $n + ') ' + (Format-EventTime $p.Time) + '  ' + $label + $detail)
        }
    }
    [void]$lines.Add('')

    # Timeline
    [void]$lines.Add('=== خط زمانی کامل آخرین اجرا ===')
    foreach ($e in $launch) {
        $line = '  ' + (Format-EventTime $e.Time) + '  ' + $e.Category + ' / ' + $e.Event
        $extras = New-Object System.Collections.ArrayList
        foreach ($k in $e.Data.Keys) {
            if ($k -eq 'exe' -or $k -eq 'args' -or $k -eq 'host') { continue }
            [void]$extras.Add($k + '=' + $e.Data[$k])
        }
        if ($extras.Count -gt 0) { $line = $line + '  [' + ($extras -join ' ') + ']' }
        [void]$lines.Add($line)
    }
    [void]$lines.Add('')
}

# ---------------------------------------------------------------------------
# 4. Machine state
# ---------------------------------------------------------------------------

[void]$lines.Add('=== وضعیت سیستم ===')
foreach ($f in Get-SystemFindings) {
    [void]$lines.Add('  ' + $f)
}
[void]$lines.Add('')

[void]$lines.Add('=== راهنمای سریع ===')
[void]$lines.Add('  - اگر «آداپتور TUN در زمان مقرر بالا نیامد» دیدید:')
[void]$lines.Add('      برنامه را ببندید، سپس در PowerShell با دسترسی Administrator اجرا کنید:')
[void]$lines.Add('        Get-NetAdapter -Name NexVPN-TUN,NexVPN-OVPN -IncludeHidden | Remove-NetAdapter -Confirm:$false')
[void]$lines.Add('      و بعد دوباره وصل شوید.')
[void]$lines.Add('  - اگر «هسته بلافاصله از کار افتاد» دیدید: متن خطای هسته در خط زمانی بالاست.')
[void]$lines.Add('  - اگر «دسترسی Administrator: ندارد» دیدید: برنامه را با Run as administrator باز کنید.')
[void]$lines.Add('')

# ---------------------------------------------------------------------------
# 5. Stage files, write the verdict, zip it
# ---------------------------------------------------------------------------

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$staging = Join-Path $env:TEMP ('NexVPN-Report-' + $stamp)
$null = New-Item -ItemType Directory -Path $staging -Force

$verdictPath = Join-Path $staging 'verdict.txt'
$lines | Out-File -LiteralPath $verdictPath -Encoding UTF8 -Force

# Diary files only. config.json and history.json are intentionally excluded —
# they carry subscription tokens.
$collected = New-Object System.Collections.ArrayList
foreach ($name in @('startup.log', 'startup.prev.log')) {
    $p = Join-Path $dataDir $name
    if (Test-Path -LiteralPath $p) {
        $dest = Join-Path $staging $name
        Copy-Item -LiteralPath $p -Destination $dest -Force
        [void]$collected.Add($dest)
    }
}

$systemPath = Join-Path $staging 'system-info.txt'
(Get-SystemFindings) | Out-File -LiteralPath $systemPath -Encoding UTF8 -Force

$zipPath = $null
if (-not $NoZip) {
    $desktop = [Environment]::GetFolderPath('Desktop')
    if (-not $desktop) { $desktop = $env:USERPROFILE }
    $zipPath = Join-Path $desktop ('NexVPN-Report-' + $stamp + '.zip')
    try {
        Compress-Archive -Path (Join-Path $staging '*') -DestinationPath $zipPath -Force
    } catch {
        $zipPath = $null
        [void]$lines.Add('هشدار: ساخت فایل zip شکست خورد — ' + $_.Exception.Message)
    }
}

# ---------------------------------------------------------------------------
# 6. Present
# ---------------------------------------------------------------------------

if ($zipPath) {
    Write-Host ''
    Write-Host 'فایل گزارش ساخته شد:' -ForegroundColor Green
    Write-Host ('  ' + $zipPath) -ForegroundColor Green
    Write-Host ''
}

Write-Host 'فایل تشخیص:' -ForegroundColor Cyan
Write-Host ('  ' + $verdictPath)
Write-Host ''

if ($summary) {
    Write-Host 'خلاصه: ' + $summary -ForegroundColor Yellow
    Write-Host ''
}

if (-not $NoNotepad) {
    try { Start-Process -FilePath 'notepad.exe' -ArgumentList ('"' + $verdictPath + '"') } catch { }
}

if ($NoNotepad) {
    # Headless use (tests, CI): emit the verdict on stdout.
    Get-Content -LiteralPath $verdictPath -Encoding UTF8 | Write-Output
}

exit 0
