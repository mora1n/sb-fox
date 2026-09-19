# <img src="frontend/public/favicon-32x32.png" alt="sb-fox" width="28" height="28"> sb-fox

[![Release](https://img.shields.io/github/v/release/mora1n/sb-fox?sort=semver)](https://github.com/mora1n/sb-fox/releases)
[![sing-box](https://img.shields.io/badge/sing--box-default%201.14.0-neutral)](https://github.com/SagerNet/sing-box)

`sb-fox` 是一个 sing-box Web 面板，用于管理节点、模板、规则集和公开订阅。

## 开始使用

安装最新版本：

```sh
curl -fsSL https://raw.githubusercontent.com/mora1n/sb-fox/main/scripts/install.sh | sh
```

启动面板：

```sh
sb-fox run
```

然后打开 <http://127.0.0.1:7878>。首次启动会在终端显示一次性 admin 密码。

直接运行 `sb-fox` 会显示帮助；需要启动服务时请使用 `sb-fox run`。

## 功能

- 导入分享链接、远程订阅、Mihomo/Surge YAML 和 sing-box 配置
- 自动识别国家，支持手动指定国家和自定义国家排序
- 管理节点、组合节点、模板、规则集和公开订阅
- 订阅源支持自动刷新，默认每 1 天抓取一次，也可以设置自定义间隔
- 使用本机 sing-box 校验、格式化并生成配置
- 支持多用户、资源上限和可轮换的共享订阅 token

## CLI

管理命令使用子命令形式，选项使用 `--` 长参数。

查看帮助：

```sh
sb-fox --help
sb-fox run --help
sb-fox daemon --help
sb-fox update --help
sb-fox status --help
sb-fox uninstall --help
sb-fox reset-admin --help
```

启动前台服务：

```sh
sb-fox run
sb-fox run --addr 127.0.0.1:7879 --data-dir ./data
```

管理 systemd 守护进程：

```sh
sudo sb-fox daemon
sudo sb-fox daemon start
sudo sb-fox daemon restart
sudo sb-fox daemon stop
sudo sb-fox daemon disable
```

`sb-fox daemon` 默认执行 `enable`：写入服务、启用服务并重启。默认守护进程位置为：

```text
socket:    /var/run/sb-fox.sock
data:      /var/lib/sb-fox
database:  /var/lib/sb-fox/sb-fox.db
templates: /var/lib/sb-fox/templates
```

更新已安装版本：

```sh
sudo sb-fox update
```

`update` 不接受运行参数；更新所需的 GitHub 凭据通过 `SB_FOX_GITHUB_TOKEN` 或 `GITHUB_TOKEN` 提供。

查看守护进程状态：

```sh
sb-fox status
```

卸载服务和二进制，保留数据：

```sh
sudo sb-fox uninstall
```

同时删除配置和数据：

```sh
sudo sb-fox uninstall --purge
```

重置 admin 密码：

```sh
sudo sb-fox reset-admin
```

开启或关闭公开注册：

```sh
sudo sb-fox --registration on
sudo sb-fox --registration off
```

已有守护进程时，这两个命令会通过内部 socket 更新设置，无需重启服务。

管理命令输出示例：

```text
✓ service enabled and restarted
  address: 127.0.0.1:7878
  data directory: /var/lib/sb-fox
```

daemon socket 内部使用 JSON 进行进程间通信，但不会作为用户命令的输出格式展示。

## 常用选项

| 选项 | 环境变量 | 默认值 |
| --- | --- | --- |
| `--addr` | `SB_FOX_ADDR` | `127.0.0.1:7878` |
| `--data-dir` | `SB_FOX_DATA_DIR` | root: `/var/lib/sb-fox`；普通用户: `~/.local/share/sb-fox` |
| `--kernel` | `SB_FOX_KERNEL` | `sing-box` |
| `--registration on\|off` | `SB_FOX_REG` | `off` |
| `--log-level error\|warn\|info\|debug` | `SB_FOX_LOG` | `info` |
| `--purge` |  | 仅用于 `uninstall`，删除配置和数据 |
| `--dev` |  | 仅提供 API，不要求嵌入前端 |
| `--version` |  | 显示版本 |

`run` 和 `daemon` 支持监听地址、数据目录、内核、注册开关和日志级别选项。`uninstall` 支持 `--purge`，`reset-admin` 支持 `--data-dir`，`update` 和 `status` 没有业务选项。

如果需要指定首次管理员密码，可以在首次启动前设置：

```sh
SB_FOX_ADMIN_PASSWORD='change-me' sb-fox run
```

查看守护进程日志：

```sh
journalctl -u sb-fox -f
```

## 模板和规则集

随包提供的模板位于 `data/templates/fakeip.json`，默认适配 sing-box `1.14.0`。模板会作为普通可编辑模板写入数据库，已有同名模板不会被启动时覆盖。

规则集支持手工 source JSON、远程 source JSON 和 binary SRS。发布时会使用当前选择的 sing-box 内核校验和编译；发布失败不会覆盖已发布的旧快照。

## 从源码构建

```sh
make frontend
make build
./sb-fox run --addr 127.0.0.1:7878 --data-dir ./data
```

检查项目：

```sh
make test
make parity
make build
sing-box check -c data/templates/fakeip.json
```

## 安全提示

`/sub/{token}/{订阅名称}` 和 `/rules/{token}/{规则集名称}.{json|srs}` 是公开入口。完整链接包含用户级共享凭据，泄露后请在设置中轮换 token。

远程订阅和规则集抓取默认拒绝私网、环回、链路本地、CGNAT、组播和云元数据地址。只有在可信网络环境中才建议开启私网抓取。

## 免责声明

本项目仅供个人学习、研究和合法合规用途。使用本项目产生的任何风险和后果均由使用者自行承担，包括但不限于配置错误、服务异常、账号或服务器被封禁、资源滥用、数据泄露、经济损失，以及违反当地法律法规、第三方服务条款或网络管理规定所产生的责任。

禁止将本项目用于网络攻击、非法访问、绕过访问控制、数据窃取、滥用代理、传播恶意内容，或任何未经授权的行为。使用者应自行确认对目标系统、节点、订阅内容和网络资源拥有合法授权，并负责保护管理员密码、订阅 token、规则集链接及其他敏感信息。

作者不对使用本项目造成的直接或间接损失承担责任，也不提供任何形式的安全承诺、可用性保证或持续技术支持。项目依赖的 sing-box、systemd、GitHub 及其他第三方软件和服务分别受其自身许可证、服务条款和隐私政策约束。

如不同意上述内容，请停止使用本项目，并删除已安装的程序及相关数据。
