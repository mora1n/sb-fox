# <img src="frontend/public/favicon-32x32.png" alt="sb-fox" width="28" height="28"> sb-fox

[![Release](https://img.shields.io/github/v/release/mora1n/sb-fox?sort=semver)](https://github.com/mora1n/sb-fox/releases)
[![sing-box](https://img.shields.io/badge/sing--box-default%201.14.0-neutral)](https://github.com/SagerNet/sing-box)

`sb-fox` 是一个轻量的 sing-box Web 面板，用于管理节点、模板、规则集和公开订阅。

它围绕日常使用流程设计：导入节点，按国家整理，选择模板，然后发布带共享 token 的 sing-box 订阅链接。

## 功能

- 从分享链接、远程订阅或已有 sing-box 配置导入节点
- 自动识别节点国家，也支持手动修正
- 按可自定义的国家热度顺序生成国家 selector
- 在设置页通过拖拽调整国家优先级
- 在简洁的 Web UI 中管理模板、节点和订阅
- 聚合手工 source JSON、远程 JSON/SRS 规则源，发布 JSON/SRS 下载链接
- 支持多用户，管理员可管理用户、重置密码并设置资源上限
- 生成可轮换共享 token 的公开订阅链接，并按订阅名称区分
- 调用本机 sing-box 校验和格式化生成配置，未安装内核时相关按钮会置灰提示

## 使用路径

1. 安装最新 release，或从源码构建 `sb-fox`。
2. 启动 `sb-fox`，也可以通过 `sb-fox --daemon` 启用守护进程。
3. 打开 `http://127.0.0.1:7878`，使用首次打印的 admin 密码登录。
4. 在“节点”中导入分享链接、远程订阅或 sing-box 配置。
5. 在“订阅”中选择模板、配置出口分组，并生成预览。
6. 开启分享订阅后，复制订阅链接到 sing-box 客户端使用。
7. 如需自托管规则集，在“规则集”中添加规则源并把生成的远程规则集片段加入模板。

## 安装

安装最新 GitHub Release：

```sh
curl -fsSL https://raw.githubusercontent.com/mora1n/sb-fox/main/scripts/install.sh | sh
```

安装脚本会下载匹配当前系统的 release 包，安装 `sb-fox` 二进制，并把种子模板复制到默认数据目录的 `templates` 子目录。安装完成后不会自动启用守护进程，只会打印下一步命令。

root 用户安装时，种子模板默认复制到 `/var/lib/sb-fox/templates`；普通用户安装时默认复制到 `~/.local/share/sb-fox/templates`。

## 运行

```sh
sb-fox
```

启动后打开 `http://127.0.0.1:7878`。

首次运行时，`sb-fox` 会创建管理员账号，并在终端打印一次性密码。默认用户名是 `admin`。

如果已有守护进程在运行，直接执行 `sb-fox` 会显示 daemon 状态并退出，不会再启动第二个同地址服务。

如需开放用户自行注册：

```sh
sudo sb-fox --reg on
```

关闭注册：

```sh
sudo sb-fox --reg off
```

没有守护进程时，`--reg on|off` 仍可作为前台启动参数使用。

## 守护进程模式

```sh
sudo sb-fox --daemon
sudo sb-fox --daemon restart
```

`--daemon` 默认等同 `--daemon enable`，会生成 systemd service，执行 `systemctl enable sb-fox` 后重启服务，并使用以下默认位置。可用命令为 `enable`、`start`、`stop`、`restart`、`disable`，其中 `enable` 会启用并重启当前服务，`disable` 会停止并禁用服务。

首次启用守护进程时，命令会在当前终端显示一次性 admin 密码。若 admin 已存在，旧密码无法再次显示，可用 `sudo sb-fox -P` 重置。

- 单实例 socket：`/var/run/sb-fox.sock`
- 数据目录：`/var/lib/sb-fox`
- 数据库：`/var/lib/sb-fox/sb-fox.db`
- 模板目录：`/var/lib/sb-fox/templates`

如果已有守护进程在运行，新的服务启动会直接失败。socket 是内部单实例锁，不提供命令行参数。

守护进程启动后，可通过命令修改注册开关，无需重启服务：

```sh
sudo sb-fox --reg on
sudo sb-fox --reg off
```

查看守护进程日志：

```sh
journalctl -u sb-fox -f
```

更新已安装版本：

```sh
sudo sb-fox -u
```

## 卸载

保留配置和数据：

```sh
sudo sb-fox --uninstall
```

同时删除配置和数据：

```sh
sudo sb-fox --uninstall --purge
```

## 模板

随包提供的模板是：

```text
data/templates/fakeip.json
```

它会以普通可编辑模板的形式写入数据库，模板名为 `fakeip`。如果数据库中已经存在同名模板，重启时不会覆盖用户修改。

该模板默认适配 sing-box `1.14.0`，并以该版本进行兼容性检查；`sb-fox` 运行时不锁定 sing-box 版本。

## 规则集

规则集模块支持按顺序聚合以下来源：

- 手工输入的 sing-box source-format JSON
- 远程 source JSON
- 远程 binary SRS

保存或手动刷新时，`sb-fox` 会使用当前用户选择的 sing-box 内核完成校验、SRS 反编译、结构去重和重新编译。任一来源失败都会中止本次发布，已经发布的旧快照不会被覆盖。

发布成功后可在面板复制 source JSON、binary SRS 链接，或直接复制 `route.rule_set` 配置片段。规则集和订阅复用同一个用户级共享 token；轮换 token 会同时撤销两类旧链接。

## 配置

常用选项：

| 选项 | 环境变量 | 默认值 |
|---|---|---|
| `--addr`, `-a` | `SB_FOX_ADDR` | `127.0.0.1:7878` |
| `--data-dir`, `-D` | `SB_FOX_DATA_DIR` | root: `/var/lib/sb-fox`；普通用户: `~/.local/share/sb-fox` |
| `--kernel`, `-k` | `SB_FOX_KERNEL` | `sing-box` |
| `--daemon [enable\|start\|stop\|restart\|disable]`, `-d ...` |  | 管理 systemd 服务，默认 `enable` |
| `--update`, `-u` |  | 更新已安装版本 |
| `--uninstall`, `-U` |  | 卸载服务和二进制 |
| `--purge`, `-p` |  | 卸载时同时删除配置和数据 |
| `--reg on\|off`, `-r on\|off` | `SB_FOX_REG` | `off` |
| `--log error\|warn\|info\|debug`, `-l ...` | `SB_FOX_LOG` | `info` |
| `--reset-admin`, `-P` |  | 重置 admin 密码并打印新随机密码 |

带默认值的字符串参数可以省略值，例如 `sb-fox -l` 会使用当前默认日志级别。

如果希望自行指定首次启动的管理员密码，可以在启动前设置 `SB_FOX_ADMIN_PASSWORD`。

忘记 admin 密码时：

```sh
sb-fox -P                # 当前用户默认数据目录
sudo sb-fox -P           # daemon /var/lib/sb-fox
sb-fox -P -D ./data      # 指定数据目录
```

## 从源码构建

```sh
make frontend
make build
./sb-fox --addr 127.0.0.1:7878 --data-dir ./data
```

常用检查：

```sh
make test
make parity
sing-box check -c data/templates/fakeip.json
```

模板兼容性检查默认使用 sing-box `1.14.0` 二进制。未安装 sing-box 时，配置生成仍可使用，校验和格式化功能会在前端置灰提示。

## 安全

`/sub/{token}/{订阅名称}` 和 `/rules/{token}/{规则集名称}.{json|srs}` 是公开入口。token 是用户级共享凭据，任何拿到完整链接的人都可以获取对应配置或规则集；如果 token 泄露，应及时轮换。

远程订阅和规则集抓取默认拒绝私网、环回和云元数据地址。规则集单源限制为 64 MiB，单次聚合原始输入总计限制为 256 MiB。只有在可信网络环境中才建议开启私网地址抓取。

## 免责声明

本项目仅供个人学习、研究和合法合规用途。使用本项目产生的任何风险和后果均由使用者自行承担，包括但不限于配置错误、服务异常、账号或服务器被封禁、资源滥用、数据泄露、经济损失以及违反当地法律法规所产生的责任。

禁止将本项目用于网络攻击、非法访问、数据窃取、滥用代理或任何未经授权的行为。作者不对使用本项目造成的直接或间接损失承担责任，也不提供任何形式的担保、承诺或技术支持。如不同意上述内容，请停止使用本项目。
