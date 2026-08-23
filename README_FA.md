<div align="center" dir="rtl">
  <img src="frontend/src/assets/images/logo.png" alt="NexVPN Pro Logo" width="130" height="130">
  <h1>نرم‌افزار NexVPN Pro</h1>
  <p><b>⚡ پیشرفته‌ترین کلاینت چند‌هسته‌ای دسکتاپ برای دور زدن فیلترینگ و حفظ حریم خصوصی</b></p>
  <p><i>تجمیع قدرتمندترین هسته‌های مسیریابی دنیا: Sing-box, Xray, Aether (MASQUE/HTTP3), WireGuard, OpenVPN و Psiphon در یک برنامه یکپارچه و فوق‌سریع.</i></p>

  <p>
    <a href="https://github.com/Rpg-420/NexVPN/releases/latest"><img src="https://img.shields.io/github/v/release/Rpg-420/NexVPN?style=for-the-badge&color=00F0FF&logo=github" alt="آخرین نسخه"></a>
    <a href="https://github.com/Rpg-420/NexVPN/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Rpg-420/NexVPN?style=for-the-badge&color=A855F7" alt="لایسنس"></a>
    <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version">
    <img src="https://img.shields.io/badge/React-18-61DAFB?style=for-the-badge&logo=react&logoColor=black" alt="React 18">
    <img src="https://img.shields.io/badge/پلتفرم-ویندوز%20%7C%20لینوکس%20%7C%20مک-10B981?style=for-the-badge" alt="Platforms">
  </p>

  <p>
    <a href="#-امکانات-و-ویژگی‌های-کلیدی">ویژگی‌های کلیدی</a> •
    <a href="#-معماری-و-موتورهای-هسته">موتورهای هسته</a> •
    <a href="#-جدول-مقایسه-پروتکل‌ها">جدول پروتکل‌ها</a> •
    <a href="#-راهنمای-نصب-و-راه‌اندازی">راهنمای نصب</a> •
    <a href="#-کامپایل-از-سورس-کد">کامپایل سورس</a> •
    <a href="#-مستندات-و-وبسایت">مستندات آنلاین</a> •
    <a href="README.md">🇬🇧 English Readme</a>
  </p>
</div>

---

<div dir="rtl">

## 🌟 معرفی پروژه

**NexVPN Pro** یک پلتفرم جامع و مدرن برای عبور از فیلترینگ شدید، سامانه‌های نظارتی DPI (تحلیل عمیق بسته‌ها)، اختلالات پروتکل و قطعی‌های اینترنت است. این نرم‌افزار با معماری ترکیبی **Go + Wails v2 + TypeScript + React 18** توسعه یافته و با ترکیب هوشمند هسته‌های مختلف، تضمین می‌کند که کاربر هیچ‌گاه پشت درهای بسته سانسور نماند.

چه با اختلال SNI، مسدودسازی TLS، اختلالات روی UDP، مسمومیت DNS یا قطع کامل ارتباطات مواجه باشید، NexVPN Pro با اسکنرهای خودکار، تکه‌تکه کردن بسته‌های TLS (فرگمنت)، و مسیریابی لایه‌ای ترافیک شما را پایدار نگه می‌دارد.

---

## 🚀 امکانات و ویژگی‌های کلیدی

### 🧠 دیسپچر چند‌هسته‌ای هوشمند (Multi-Engine Core)
- مدیریت و سوئیچینگ خودکار میان هسته‌های **Sing-box**، **Xray-core**، **Aether (MASQUE/HTTP3)**، **Cloudflare WARP / WireGuard**، **OpenVPN** و **Psiphon**.
- سوئیچ اضطراری خودکار (Auto-Failover): در صورت قطع شدن یک سرور، اتصال بلافاصله و بدون وقفه به سریع‌ترین نود سالم بعدی منتقل می‌شود.

### 📡 مرکز اکتشاف و اسکنرهای هوشمند (Discovery Hub)
- **ربات استخراج پروکسی تلگرام**: دریافت، رمزگشایی و تست تاخیر زنده پروکسی‌های عمومی MTProto و کانفیگ‌های V2Ray از کانال‌های معتبر تلگرامی.
- **اسکنر IP تمیز کلودفلر (Clean IP Scanner)**: اسکن فوق‌سریع و چندنخی رنج‌های مختلف IPv4 کلودفلر با بررسی دقیق پینگ، جیتر (Jitter) و درصد پکت‌لاس.
- **استودیوی فرگمنت TLS (Fragment Studio)**: محیط بصری برای تکه‌تکه کردن پکت ClientHello و تنظیم تاخیر میلی‌ثانیه‌ای جهت عبور آسان از سد فیلترینگ SNI و DPI.
- **هاب کانفیگ‌های رایگان**: بروزرسانی لحظه‌ای کانفیگ‌های رایگان با برچسب‌گذاری تفکیک‌شده برای اپراتورهای ایرانی (*همراه اول، ایرانسل، رایتل، شاتل، مخابرات و...*).
- **بهینه‌ساز هوشمند اپراتور (Smart ISP Optimizer)**: تشخیص خودکار اپراتور اینترنت شما و اعمال خودکار تنظیمات فرگمنت، MTU و آی‌پی تمیز مناسب همان اپراتور.

### 🛡️ امنیت پیشرفته و یکپارچگی سیستمی
- **کیل‌سوئیچ لایه صفر (Zero-Leak Kill Switch)**: قفل دیواره آتش سیستم‌عامل (*ویندوز Netsh، لینوکس iptables/nftables، مک pfctl*) برای جلوگیری از نشت تصادفی آی‌پی واقعی شما هنگام قطعی تونل.
- **سپر ضد نشت DNS (DNS Leak Shield)**: مسیریابی اجباری کوئری‌های DNS از ریزالورهای رمزگذاری‌شده (*Cloudflare, Quad9, Google, Custom DoH/DoT*).
- **تفکیک ترافیک برنامه‌ها (Process-Based App Split Tunneling)**: انتخاب فایل‌های اجرایی برنامه‌ها (مانند بازی‌های آنلاین، مرورگرهای بانکی یا تلگرام) جهت عبور مستقیم یا عبور از فیلترشکن.
- **پزشک هوشمند شبکه (Network Doctor & Emergency Repair)**: تعمیر و نوسازی کامل شبکه دسکتاپ تنها با یک کلیک (خالی کردن کش DNS، ریست استک Winsock، پاکسازی پروکسی‌های رهاشده و بستن پروسس‌های مخفی هسته‌ها).
- **سرویس Polkit لینوکس**: اجرای کلاینت در پس‌زمینه لینوکس با دسترسی `cap_net_admin` بدون نیاز به تکرار پسورد روت.

### ⚡ اکوسیستم ابزارهای بهره‌وری
- **کنترل از راه دور موبایل (Mobile Remote Companion)**: امکان اتصال گوشی هوشمند به نسخه دسکتاپ از طریق اسکن QR کد و پین ۶ رقمی در بستر شبکه محلی (Wi-Fi) جهت قطع و وصل و مشاهده آمار.
- **اشتراک‌گذاری پروکسی در شبکه محلی (LAN Share)**: اشتراک تونل امن برای تلویزیون هوشمند، کنسول‌های بازی و سایر لپ‌تاپ‌های شبکه بدون نیاز به نصب نرم‌افزار روی آنها.
- **همگام‌سازی محلی نودها (P2P Nearby Sync)**: کشف کلاینت‌های دیگر نکس‌وی‌پی‌ان در شبکه محلی و تبادل آنی کانفیگ‌ها و آی‌پی‌های تمیز.
- **پشتیبان‌گیری ابری و والت رمزگذاری‌شده**: خروجی و بازیابی با رمزگذاری امن AES-GCM و امکان همگام‌سازی با GitHub Gist.
- **حالت گیمینگ و ویجت شناور دسکتاپ**: شتاب‌دهنده پکت‌های UDP با جیتر پایین همراه با ویجت مینیاتوری همیشه در دسترس روی صفحه دسکتاپ.
- **اسکنر QR کد از روی صفحه**: استخراج خودکار کانفیگ با اسکن بخشی از مانیتور، فایل تصویری یا وبکم.

---

## 📐 معماری و موتورهای هسته

NexVPN ترافیک شما را در سه سطح مختلف مدیریت می‌کند:

1. **موتور Aether (پروتکل‌های نسل جدید MASQUE و gool)**:
   - تونل‌بندی UDP بر بستر پروتکل HTTP/3.
   - ایجاد تونل‌های چندلایه کلودفلر (WARP-in-WARP) با ۵ سطح اسکن تطبیقی (*Turbo, Balanced, Thorough, Stealth, Ironclad*).
2. **موتور Sing-box**:
   - پیشرفته‌ترین هسته مدرن با پشتیبانی از VLESS Reality, Trojan, Hysteria 2, TUIC v5 و WireGuard.
   - آداپتور سیستمی TUN با کمترین سربار پردازشی و مصرف رم.
3. **موتور Xray-core**:
   - پروتکل‌های استاندارد V2Ray نظیر VMess, VLESS با ترانسپورت‌های gRPC و WebSocket.
4. **موتور OpenVPN**:
   - اتصال به فایل‌های پیکربندی سازمانی و سنتی `.ovpn` با مسیریابی لایه ۳.
5. **موتور Psiphon**:
   - متد اختصاصی SSH مبهم‌سازی‌شده جهت اتصال اضطراری در زمان خاموشی سراسری یا فیلترینگ شدید پروتکل‌ها.

---

## 📊 جدول مقایسه پروتکل‌ها

| نام پروتکل | هسته اجرایی | ساختار امنیت و ترانسپورت | امتیاز عبور از DPI | شتاب UDP | کاربرد پیشنهادی |
| :--- | :--- | :--- | :---: | :---: | :--- |
| **VLESS Reality** | Sing-box / Xray | TCP, gRPC, WebSocket | 🟢 **۹۹٪** | ✅ | اختلال شدید SNI و شبکه‌های مسدود |
| **Aether MASQUE** | Aether Native | HTTP/3 UDP / کلودفلر تو در تو | 🟢 **۹۸٪** | 🚀 توربو | قطعی سراسری اینترنت بین‌الملل |
| **Hysteria 2** | Sing-box | پروتکل QUIC اختصاصی Brutal | 🟢 **۹۶٪** | 🚀 فوق‌العاده | اینترنت‌های موبایل همراه با پکت‌لاس بالا و گیمینگ |
| **TUIC v5** | Sing-box | UDP بر بستر QUIC | 🟢 **۹۵٪** | 🚀 فوق‌العاده | دانلود پرسرعت و استریم ویدیو 4K |
| **WARP / WireGuard** | Sing-box / Native | UDP + اسکنر IP تمیز | 🟡 **۸۸٪** | ✅ | استفاده روزمره و مخفی‌سازی IP |
| **Trojan / VMess** | Sing-box / Xray | TLS, WS, gRPC | 🟡 **۹۰٪** | ✅ | سرورهای پشت CDN و کلودفلر |
| **OpenVPN** | OpenVPN | لایه ۳ TUN سنتی | 🟡 **۸۰٪** | ⚠️ معمولی | سرورهای دانشگاهی و اداری |
| **Psiphon SSH** | Psiphon | سشن SSH مبهم‌شده | 🟢 **۹۵٪** | ⚠️ فقط TCP | اتصال پشتیبان در زمان قطعی کامل |

---

## 🛠️ راهنمای نصب و راه‌اندازی

### 🪟 ویندوز (Windows 10 / 11)
1. فایل نصب‌کننده `NexVPN-Setup.exe` یا نسخه پرتابل `NexVPN-windows-amd64.zip` را از بخش [Releases](https://github.com/Rpg-420/NexVPN/releases/latest) دانلود کنید.
2. برنامه را نصب و اجرا کنید. درایور Wintun و سینی ویندوز (System Tray) به صورت خودکار فعال می‌شوند.

### 🐧 لینوکس (Ubuntu / Debian / Arch / Fedora)
1. فایل فشرده `NexVPN-linux-amd64.tar.gz` را دانلود و استخراج نمایید:
   ```bash
   tar -xvf NexVPN-linux-amd64.tar.gz
   cd NexVPN
   ```
2. **اقدام ضروری:** اسکریپت دسترسی بدون پسورد را یک بار با دسترسی روت اجرا کنید:
   ```bash
   sudo ./install_linux.sh
   ```
   > 💡 *این اسکریپت با تنظیم Polkit و اعطای دسترسی `cap_net_admin` به هسته‌ها، نیاز به وارد کردن پسورد روت در هر بار اتصال را به کلی حذف می‌کند.*

### 🍏 مک (macOS)
1. فایل `NexVPN-darwin-universal.dmg` را دانلود کرده و فایل `NexVPN.app` را درون پوشه `Applications` بیندازید.

---

## 🏗️ کامپایل از سورس کد

برای بیلد اختصاصی پروژه به ابزارهای زیر نیاز دارید:
- زبان [Go نسخه 1.22 یا بالاتر](https://golang.org/doc/install)
- [Node.js نسخه 20 به بالا](https://nodejs.org/)
- ابزار [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0`)

```bash
# ۱. کلون کردن مخزن
git clone https://github.com/Rpg-420/NexVPN.git
cd NexVPN

# ۲. نصب وابستگی‌های رابط کاربری
cd frontend && npm install && cd ..

# ۳. کامپایل نسخه نهایی
wails build -clean
```
فایل اجرایی نهایی در مسیر `build/bin/NexVPN.exe` (ویندوز) یا `build/bin/NexVPN` (لینوکس) قرار می‌گیرد.

---

## 📚 مستندات و وبسایت

برای مطالعه آموزش‌های گام به گام و استفاده از ابزارهای آنلاین به سایت ما مراجعه کنید:

- 🌐 **[وبسایت رسمی نکس‌وی‌پی‌ان و ابزارهای آنلاین](https://Rpg-420.github.io/NexVPN/)**
- 📖 [راهنمای جامع کاربری (فارسی)](https://Rpg-420.github.io/NexVPN/user-guide-fa.html) | [User Guide (English)](https://Rpg-420.github.io/NexVPN/user-guide-en.html)
- 💻 [راهنمای توسعه‌دهندگان (فارسی)](https://Rpg-420.github.io/NexVPN/developer-guide-fa.html) | [Developer Guide (English)](https://Rpg-420.github.io/NexVPN/developer-guide-en.html)

---

## 🤝 مشارکت در توسعه

ما از هرگونه مشارکت، گزارش باگ و ایده جدید استقبال می‌کنیم! لطفاً قبل از ارسال PR، فایل‌های [راهنمای مشارکت](.github/CONTRIBUTING.md) و [مرام‌نامه جامعه](.github/CODE_OF_CONDUCT.md) را مطالعه فرمایید.

---

## 📄 لایسنس
این پروژه به صورت متن‌باز تحت لایسنس **[MIT License](LICENSE)** منتشر شده است.
</div>
