<div align="center" dir="rtl">
  <img src="frontend/src/assets/images/logo.png" alt="NexVPN Logo" width="120" height="120">
  <h1>NexVPN Pro</h1>
  <p><b>نسل جدید نرم‌افزارهای ضد فیلترینگ با معماری چند-هسته‌ای</b></p>
  <p>
    <a href="https://github.com/Rpg-420/NexVPN/releases/latest"><img src="https://img.shields.io/github/v/release/Rpg-420/NexVPN?style=for-the-badge&color=00F0FF" alt="Latest Release"></a>
    <a href="https://github.com/Rpg-420/NexVPN/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Rpg-420/NexVPN?style=for-the-badge&color=A855F7" alt="License"></a>
    <img src="https://img.shields.io/badge/Platform-Win%20%7C%20Mac%20%7C%20Linux-10B981?style=for-the-badge" alt="Platforms">
  </p>
  <p>
    <a href="#ویژگیهای-کلیدی">ویژگی‌ها</a> •
    <a href="#سه-موتور-قدرتمند">موتورها</a> •
    <a href="#نصب-و-راهاندازی">نصب</a> •
    <a href="#مستندات">مستندات</a> •
    <a href="README.md">🇺🇸 English Readme</a>
  </p>
</div>

---

<div dir="rtl">

نرم‌افزار **NexVPN Pro** صرفاً یک کلاینت ساده نیست. این برنامه یک پلتفرم هوشمند، کراس‌پلتفرم و فوق‌پیشرفته است که با زبان‌های **Go، Wails و React** مهندسی شده است. با ادغام قدرتمندترین هسته‌های مسیریابی جهان در یک رابط کاربری یکپارچه، این نرم‌افزار عبور از خشن‌ترین سیستم‌های فیلترینگ (DPI) را تضمین می‌کند.

## ✨ ویژگی‌های کلیدی

- **🧠 سیستم تغییر هسته هوشمند:** سوییچ یکپارچه و بدون قطعی بین هسته‌های Aether (MASQUE)، Sing-box و OpenVPN.
- **🛡️ پزشک هوشمند شبکه:** تشخیص خودکار قطعی‌ها، ریست کردن آداپتورهای گیرکرده (TUN)، پاکسازی کش DNS و حذف پروکسی‌های رها شده در سیستم.
- **🎯 تفکیک ترافیک برنامه‌ها (Split Tunneling):** برنامه‌های بانکی یا بازی‌های آنلاین خود را از VPN مستثنی کنید! کافیست فایل اجرایی برنامه را انتخاب کنید تا ترافیک آن مستقیماً از اینترنت آزاد عبور کند.
- **⚡ کیل‌سوئیچ بدون نشتی:** یکپارچگی عمیق با سیستم‌عامل تضمین می‌کند که در صورت قطع شدن تونل، آی‌پی واقعی شما هرگز لو نمی‌رود.
- **🖥️ سازگاری عمیق با سیستم‌عامل:**
  - **لینوکس:** اجرای بدون پسورد از طریق Polkit و مسیریابی `cap_net_admin`.
  - **ویندوز:** مدیریت هوشمند رجیستری و درایور Wintun.
  - **مک:** استارت‌آپ اتوماتیک با LaunchAgent و کنترل فایروال از طریق `pfctl`.

## 🚀 سه موتور قدرتمند

شما در NexVPN محدود به یک پروتکل نیستید. ترافیک شما بهینه‌ترین مسیر را از بین هسته‌های زیر پیدا می‌کند:

### ۱. موتور Aether (MASQUE / HTTP3)
پرچمدار ضد سانسور! این هسته از پروتکل HTTP/3 بر بستر UDP استفاده کرده و از سیستم تونل‌های تودرتو (**WARP-in-WARP**) پشتیبانی می‌کند. دارای ۵ حالت اسکنر آی‌پی هوشمند (از Turbo تا Ironclad) است که به طور تهاجمی آی‌پی‌های تمیز Cloudflare را شکار می‌کند.

### ۲. موتور Sing-box
استاندارد جهانی برای اتصال به سرورهای مدرن. این هسته از **VLESS Reality، Trojan، Hysteria2 و WireGuard** پشتیبانی می‌کند و دارای سیستم قدرتمند TUN و Smart DNS برای جلوگیری از آلودگی DNS است.

### ۳. موتور OpenVPN
مسیریابی اصیل لایه ۳ برای پیکربندی‌های کلاسیک. این هسته به طور کامل با فایروال سیستم یکپارچه شده تا از نشت دیتا جلوگیری کند.

---

## 🛠️ نصب و راه‌اندازی

جدیدترین نسخه‌های کامپایل شده برای ویندوز، مک و لینوکس را از صفحه [Releases](https://github.com/Rpg-420/NexVPN/releases) دانلود کنید.

> [!IMPORTANT]
> **نکته بسیار مهم برای کاربران لینوکس:**
> پس از دانلود و استخراج فایل‌ها، برای اینکه برنامه بتواند بدون درخواست مکرر پسورد روت در پس‌زمینه اجرا شود و ترافیک را مدیریت کند، **باید** یک بار اسکریپت زیر را اجرا کنید:
> ```bash
> sudo ./install_linux.sh
> ```

## 🏗️ بیلد از روی سورس‌کد

پیش‌نیازها:
- [Go](https://golang.org/doc/install) نسخه 1.23 به بالا
- [Node.js](https://nodejs.org/) نسخه 20 به بالا
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# Clone the repository
git clone https://github.com/Rpg-420/NexVPN.git
cd NexVPN

# Install Wails CLI if you haven't already
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build for your operating system
wails build -clean
```
فایل اجرایی بیلد شده در پوشه `build/bin` قرار می‌گیرد.

## 📚 مستندات و آموزش‌ها

ما آموزش‌های کاملی را برای کاربران عادی و توسعه‌دهندگان آماده کرده‌ایم. از سایت رسمی گیت‌هاب پیج ما دیدن کنید:

- 🌐 **[سایت رسمی و مستندات NexVPN](https://Rpg-420.github.io/NexVPN/)**

لینک‌های مستقیم:
- 📖 [راهنمای کاربری (فارسی)](https://Rpg-420.github.io/NexVPN/user-guide-fa.html)
- 💻 [مستندات توسعه‌دهندگان (فارسی)](https://Rpg-420.github.io/NexVPN/developer-guide-fa.html)

## 📄 لایسنس
این پروژه متن‌باز است و تحت لایسنس [MIT](LICENSE) منتشر شده است.
</div>
