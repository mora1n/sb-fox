# sb-fox

[![Release](https://img.shields.io/github/v/release/mora1n/sb-fox?sort=semver)](https://github.com/mora1n/sb-fox/releases)
[![sing-box](https://img.shields.io/badge/sing--box-1.13.14-neutral)](https://github.com/SagerNet/sing-box)

`sb-fox` 是一个轻量的 sing-box Web 面板，用于管理节点、模板和公开订阅分组。

它围绕日常使用流程设计：导入节点，按国家整理，选择模板，然后发布带 token 的 sing-box 订阅链接。

## 功能

- 从分享链接、远程订阅或已有 sing-box 配置导入节点
- 自动识别节点国家，也支持手动修正
- 按可自定义的国家热度顺序生成国家 selector
- 在设置页通过拖拽调整国家优先级
- 在简洁的 Web UI 中管理模板、节点和订阅分组
- 生成可轮换 token 的公开订阅链接
- 调用本机 sing-box 校验和格式化生成配置
- 支持在设置页修改面板显示名称

## 安装

安装最新 GitHub Release：

```sh
curl -fsSL https://raw.githubusercontent.com/mora1n/sb-fox/main/scripts/install.sh | sh
```

安装脚本会下载匹配当前系统的 release 包，安装 `sb-fox` 二进制，并默认把种子模板复制到 `./data/templates`。

## 运行

```sh
sb-fox --addr :8080 --data-dir ./data
```

启动后打开 `http://127.0.0.1:8080`。

首次运行时，`sb-fox` 会创建管理员账号，并在终端打印一次性密码。默认用户名是 `admin`。

## 模板

随包提供的模板是：

```text
data/templates/fakeip.json
```

它会以普通可编辑模板的形式写入数据库，模板名为 `fakeip`。如果数据库中已经存在同名模板，重启时不会覆盖用户修改。

该模板面向 sing-box `1.13.14`。

## 配置

常用选项：

| 选项 | 环境变量 | 默认值 |
|---|---|---|
| `--addr` | `SB_FOX_ADDR` | `:8080` |
| `--data-dir` | `SB_FOX_DATA_DIR` | `./data` |
| `--kernel` | `SB_FOX_KERNEL` | `sing-box` |

如果希望自行指定首次启动的管理员密码，可以在启动前设置 `SB_FOX_ADMIN_PASSWORD`。

## 从源码构建

```sh
make frontend
make build
./sb-fox --addr :8080 --data-dir ./data
```

常用检查：

```sh
make test
make parity
sing-box check -c data/templates/fakeip.json
```

模板兼容性检查建议使用 sing-box `1.13.14` 二进制。

## 安全

`/sub/{token}` 链接是公开下载入口。任何拿到 token 的人都可以下载生成配置；如果 token 泄露，应及时轮换。

远程订阅抓取默认拒绝私网、环回和云元数据地址。只有在可信网络环境中才建议开启私网地址抓取。

## 免责声明

本项目仅供个人学习、研究和合法合规用途。使用本项目产生的任何风险和后果均由使用者自行承担，包括但不限于配置错误、服务异常、账号或服务器被封禁、资源滥用、数据泄露、经济损失以及违反当地法律法规所产生的责任。

禁止将本项目用于网络攻击、非法访问、数据窃取、滥用代理或任何未经授权的行为。作者不对使用本项目造成的直接或间接损失承担责任，也不提供任何形式的担保、承诺或技术支持。如不同意上述内容，请停止使用本项目。
