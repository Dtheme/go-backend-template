# 第三方 Skill 来源记录

- 名称：go-development
- 来源：https://github.com/netresearch/go-development-skill
- 版本：1.15.1（`plugin.json` / `SKILL.md` frontmatter）
- 引入日期：2026-09-05
- 引入方式：从发布包 `go-development-skill-main` 复制 `skills/go-development/` 全部内容与两份许可证
- `SKILL.md` SHA-256：`901fcc34f7181e51bbb6853eea0c047273b1b91ff69b69c031ca76b202e2831e`
- 许可证：MIT（代码与脚本）AND CC-BY-SA-4.0（文档），原文见同目录 `LICENSE-MIT`、`LICENSE-CC-BY-SA-4.0`
- 本地改动：无（目录内容与上游一致，仅新增本文件）
- 脚本副作用：`scripts/verify-go-project.sh` 只读检查（`go vet` 与文件存在性），不修改文件、不联网

## 在本模板中的定位

`go-development` 是 Go 工程实践参考，不是本工程的规则来源。冲突时以 `.ai/ai-rules.md` 为准：

| 上游内容 | 本模板处理 |
| --- | --- |
| `references/testing.md`、`architecture.md`、`api-design.md`、`logging.md`、`contracts-and-invariants.md`、`fuzz-testing.md`、`modernization.md`、`dependencies.md` | 适用，编码与测试时按需加载 |
| `references/linting.md`、`makefile.md`、`mutation-testing.md`、`single-build-release.md`、`lefthook-template.md` | 可选；`golangci-lint`、`govulncheck` 未安装时 `make lint` / `make vuln` 自动跳过 |
| `references/cron-scheduling.md`、`resilience.md`、`docker.md`、`ldap.md` | 仅当业务引入对应能力时参考 |
| `references/branch-protection.md`、`reusable-workflows.md`、`awesome-go-submission.md` | 与本模板无关，不使用 |
| frontmatter `compatibility: Requires golangci-lint, docker` | 本模板不强制；缺失时相关目标跳过 |
| 「For reviews, invoke security-audit / enterprise-readiness / github-project」 | 本模板未引入这些 Skill，忽略 |
| Quality Gates 中的 `golangci-lint`、`staticcheck`、`govulncheck` | 作为可选加强项，`make check` 不依赖 |

## 与 ai-rules.md 的已知冲突（一律以 ai-rules.md 为准）

| 上游内容 | 本模板规则 | 处理 |
| --- | --- | --- |
| Consistency：`Config precedence: defaults < config file < env vars < flags` | 配置只来自环境变量，无配置文件与 flag | 忽略上游层级，只保留 defaults < env vars |
| Testing：`Always use t.Parallel()` | 只在无共享可变状态且有明显收益时用 `t.Parallel()`，默认串行 | 按需，不强制 |
| References 段首行 `ls lefthook.yml ... || echo "Add lefthook"` | 本模板不使用 git hook，门禁走 Makefile | 读到该行不执行、不提示安装 lefthook |
| Quality Gates 的 `golangci-lint`、`staticcheck`、`govulncheck` | `make lint` / `make vuln` 可选，`make check` 不依赖 | 已安装则作为加强项运行 |
| frontmatter `description` 触发词（cron、Docker API、LDAP、golangci-lint） | 加载时机由 ai-rules.md Step 5 指定：编码与自验时按需读 references | 不依赖自动触发 |

## checkpoints.yaml 与 evals 的处理

上游附带 `checkpoints.yaml`（26 条机械检查加 3 条 LLM review）与 `evals/evals.json`（22 条），原样保留但**不接入任何门禁**。原因：其中 GD-02（要求 `go.sum`，本模板零依赖时不生成）、GD-05（`.golangci.yml`）、GD-21（强制 `t.Parallel()`）、GD-23（`make all`）、GD-24（`-coverprofile`）、GD-26（`lefthook.yml`）与本模板的取舍冲突；evals 中 LDAP、cron、Docker 三类断言与本模板无关。需要时可挑选单条手动执行，不作为交付判断。

## 完整性摘要

下表由 `shasum -a 256` 对上游复制进来的全部文件生成（不含本文件），下游工程可用同一命令重新核对是否被改动：

```
4b34c2a0857a7b24ddd66ba7449c756c5de4cc2ad528744624099ecc3731b52e  LICENSE-CC-BY-SA-4.0
6c29a3abb028408de1bd94979fa3150b8a540712517d9c6909b264e402f1f3b4  LICENSE-MIT
901fcc34f7181e51bbb6853eea0c047273b1b91ff69b69c031ca76b202e2831e  SKILL.md
e9bc21981bcc3f92563f1e0903e77b632a971eb0636ecfeba86edf82db3d9be4  checkpoints.yaml
54b6af9bcbfe9893ad10363a3a3ccb5c81a4b944adb27d928c778aaf198b804b  evals/evals.json
4395cb0792d9927ae0064948daef3c1ecbe2b166fc6edb4cff05f0dfc8772f42  references/api-design.md
de6387bb1501f50c1c6eef597da21a44801979526cad4f8d521fb39430ac0b9e  references/architecture.md
2f4efa4728aab7ef217fa7b66caaafb8dd1b83fd333e1049cc490e4984ead36a  references/awesome-go-submission.md
43bc047d261f5d45e908ed39f81d36350e705b353b57d261730e9c327d7a0b73  references/branch-protection.md
548b7f07ff6d3132e6b542082c468f0f416e1ce381542adf481ebdd50785e0e6  references/contracts-and-invariants.md
4e8efa7cc9fba679a5407aa36ab3c5eb9b7ad6a9d53adc899bc79ede61440c09  references/cron-scheduling.md
c57b3deb29160441f1732f825eab09c98d9b2f694973aa7754d4b657e4decb23  references/dependencies.md
5cfc74ec07a7a39308ef4e77d05416667026dda85ae33f4b4ef4dbcc0694ad44  references/docker.md
aa1e64c41b9cccd878c8309e4af5c17d170c84b6a66011f196852792d62484d3  references/fuzz-testing.md
a7ed1098aed324f1a1de8c93f52388646c6c9962d06ed87ebd3152fdf544acde  references/ldap.md
24f70d620c30bb9236be5d276ef632547710126cca2b178c64c67a757915d847  references/lefthook-template.md
aac1bec083429e536f184f3ac46f32dfc6151b69810a646e644df18bd200fb82  references/linting.md
9a607964e6139fea13743fd8e36dfd3b7b71f8149391aa50647e6d2c5107b830  references/logging.md
435f3d9ce5a2aab2136805ede63c6cb49e250f3b3e1defb3fe1b410da0b4569d  references/makefile.md
4950efce11c6f341c2947d80e217ec2a90cfd4713d24dc0d9d5093d77e50728e  references/modernization.md
c21cce0453ae2a1c2cae8aae6dd70c92261624be9fe111d3bdc5cef1a7a8e8b6  references/mutation-testing.md
595263bbaf2d4ef9c1a6b37eb5c7b53708619973b0b75df9464d7be4723641a5  references/resilience.md
f30e0bdf411407840017435854d485286311080cccadc9c8912b479740d12bc4  references/reusable-workflows.md
3f545f8a2e1b61ff0150c2a091ea9879c1008dc3df83cd62b4563c73765fe739  references/single-build-release.md
befc5d6f2d31faa145a0e548167e2c456b468fc7fd828b75dd3dbba83b74001f  references/testing.md
f3510f6b419076f43e77b4eab87ff63e0135ef3abb24c43f963e2e472ea2dcf6  scripts/verify-go-project.sh
```

## 停用与升级

- 停用：删除本目录，并从 `.ai/ai-rules.md` 的「Skills」表移除 `go-development` 行
- 升级：下载新版本后整目录替换，更新本文件的版本、SHA-256 与日期；本地不做修改，避免升级冲突
