// 桌面单文件运行支持：锚定可执行文件目录、挑选空闲端口、调起系统默认浏览器。
// 仅在 Windows 双击场景生效（DATA_DIR 未显式设置时），服务器 / 容器部署不受影响。
package platform

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

// IsDesktopMode 判断是否按「桌面双击运行」处理：Windows 且未显式指定 DATA_DIR。
func IsDesktopMode() bool {
	return runtime.GOOS == "windows" && os.Getenv("DATA_DIR") == ""
}

// ExeDir 返回可执行文件所在目录；取不到时退回当前工作目录。
// 双击运行时工作目录可能是 C:\Windows\System32，数据目录必须锚定到 exe 所在目录，
// 否则账本会落到系统目录或随版本被替换丢失。
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// PickedPort 从 start 起向上找第一个可监听端口，最多尝试到 max；全被占用时返回 start。
func PickedPort(start, max int) int {
	for p := start; p <= max; p++ {
		if portFree(p) {
			return p
		}
	}
	return start
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// OpenBrowser 用系统默认浏览器打开 url；失败只返回错误，不影响服务本身。
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
