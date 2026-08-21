# راهنمای توسعه‌دهندگان NexVPN Pro

پروژه NexVPN با استفاده از **Wails**، **React** و زبان **Go** توسعه یافته است. معماری این برنامه کاملاً ماژولار بوده و برای پشتیبانی از چندین موتور مسیریابی مختلف و یکپارچگی عمیق با سیستم‌عامل‌های مختلف طراحی شده است.

## ساختار پوشه‌ها
- `frontend/`: کدهای فرانت‌اند (React + TypeScript).
- `internal/`: منطق بک‌اند (Go).
  - `config/`: مدیریت استیت‌ها، ذخیره‌سازی و ساختار داده‌ها.
  - `core/`: موتور اصلی (`engine.go`, `dispatcher.go`)، شروع خودکار، کیل‌سوئیچ و مدیریت باینری‌ها.
  - `parser/`: پارسرهای Split tunneling، تولیدکنندگان کانفیگ و سیستم پزشک شبکه (`doctor.go`).
  - `protocols/`: آداپتورهای پروتکل‌ها (`interface.go`, `singbox.go`, `aether.go`, `openvpn.go`).
  - `sysproc/`: مدیریت پروسس‌ها در سطح سیستم‌عامل و Job Objectها (برای ویندوز).
  - `sysproxy/`: تغییر پروکسی سیستمی (`networksetup` در مک، رجیستری در ویندوز، `gsettings`/D-Bus در لینوکس).
- `install_linux.sh`: اسکریپت کانفیگ Polkit و `setcap` برای اجرای بدون پسورد در لینوکس.

## معماری موتورهای چند-هسته‌ای (Multi-Core)
برنامه NexVPN به یک هسته ثابت محدود نیست. در فایل `internal/protocols/interface.go`، اینترفیس `ProtocolAdapter` به این شکل تعریف شده است:
```go
type ProtocolAdapter interface {
    Name() config.Protocol
    Start(ctx context.Context, configContent string) error
    Stop() error
    GetStats() (int64, int64, error)
    IsRunning() bool
    SetLogger(fn LoggerFunc)
    SetOnExit(fn OnExitFunc)
}
```
هنگامی که کاربر روی اتصال کلیک می‌کند، فایل `engine.go` نوع پروتکل را تشخیص داده و آن را به آداپتور مربوطه (`AetherAdapter`, `SingBoxAdapter` یا `OpenVPNAdapter`) محول می‌کند.

## افزودن یک هسته/پروتکل جدید
۱. **ایجاد آداپتور**: فایل `internal/protocols/mycore.go` را بسازید و اینترفیس `ProtocolAdapter` را پیاده‌سازی کنید.
۲. **ثبت آداپتور**: آن را به `internal/protocols/interface.go` و `internal/core/dispatcher.go` اضافه کنید.
۳. **فرانت‌اند**: تایپ‌های TypeScript را در `frontend/src/types/index.ts` بروزرسانی کرده و یک صفحه React برای مدیریت کانفیگ‌های آن بسازید.
۴. **مدیریت باینری**: لینک دانلود و بررسی هش باینری هسته را به `internal/core/binary_downloader.go` اضافه کنید.

## یکپارچه‌سازی Polkit در لینوکس
برای ایجاد TUN و دستکاری iptables در لینوکس معمولاً به دسترسی روت نیاز است. برای جلوگیری از نمایش مداوم پنجره درخواست پسورد (sudo/GNOME) در NexVPN از روش‌های زیر استفاده شده است:
۱. استفاده از قابلیت‌های کرنل (`setcap cap_net_admin,cap_net_bind_service=+ep`) برای باینری‌ها.
۲. ایجاد فایل Polkit Policy (`/usr/share/polkit-1/actions/dev.nexvpn.policy`) که مسیر دقیق باینری‌ها را برای اجرای Passwordless مجاز می‌کند.
۳. پاک کردن متغیر `DBUS_SYSTEM_BUS_ADDRESS` قبل از اجرای هسته‌ها (مانند sing-box) تا از تریگر شدن هشدارهای امنیتی NetworkManager D-Bus جلوگیری شود و سیستم با آرامش کامل از `netlink` استفاده کند.

## بیلد و توسعه
```bash
# محیط توسعه با قابلیت Hot-Reload
wails dev

# بیلد نسخه نهایی (Production)
wails build -clean -platform windows/amd64,linux/amd64,darwin/arm64
```
