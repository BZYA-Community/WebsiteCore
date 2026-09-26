# 文档索引

[English](README.md) | 简体中文

本目录汇总了 PaoPao 的开发文档、部署文档、API 文档资源、设计提案以及讨论性资料。

如果你想先从项目主入口开始，建议优先阅读：

- [../README.md](../README.md) - 项目总览
- [INSTALL_ZH.md](INSTALL_ZH.md) - 安装与本地部署指南

## 文档分区

| 分区 | 说明 |
| --- | --- |
| [INSTALL_ZH.md](INSTALL_ZH.md) | 安装与本地部署指南 |
| [CHANGELOG.md](CHANGELOG.md) | 版本变更记录（含上游 paopao-ce 历史） |
| [features-status.md](features-status.md) | 功能项成熟度与支持状态 |
| [openapi/](openapi/) | 导出的 API 文档资源，包含生成的 OpenAPI 文件与静态文档产物 |
| [proposal/](proposal/) | 产品设计、功能提案与实现思路说明 |
| [deploy/](deploy/) | 本地、云平台部署参考文档（Kubernetes 部分为占位） |
| [discuss/](discuss/) | 偏讨论性质的文档、记录与相关资料 |

## 推荐阅读路径

### 新贡献者

1. 先阅读 [../README.md](../README.md)
2. 再参考 [INSTALL_ZH.md](INSTALL_ZH.md)
3. 最后浏览 [proposal/](proposal/) 了解产品定位与功能方向

### 运维与自部署使用者

1. 从 [INSTALL_ZH.md](INSTALL_ZH.md) 开始
2. 继续阅读 [deploy/README_ZH.md](deploy/README_ZH.md)
3. 再进入与你目标环境对应的平台部署文档

### API 与集成开发

1. 先查看 [openapi/](openapi/)
2. 如果接口行为与产品设计相关，再结合 [proposal/](proposal/) 理解背景

## 说明

- `openapi/` 更偏向生成产物目录，而不是纯手写说明文档。
- `proposal/` 中既有历史设计稿，也有仍具参考价值的设计说明，因此不一定全部代表最终实现。
- `discuss/` 中可能包含更偏工作记录或社区讨论性质的内容。
