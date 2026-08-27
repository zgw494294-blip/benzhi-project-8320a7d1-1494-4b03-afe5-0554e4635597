# 岩芯不可逆取样保全服务

本项目为岩芯保管员、取样实验员和地质研究复核员提供不可逆切割取样的全过程治理。服务登记岩芯及禁止切割区段，执行申请预检、复核退回、授权、实际切割、余样核验，并在通过后冻结来源链并签发可验真的取样凭据。

服务在同一主流程内提供可审计的岩芯约束修订与影响预览、案卷字段级修订历史、可复算预检快照、复核问题闭环、不可变授权清单、强幂等执行回执以及余样核验尝试台账。可用区段查询会同时解释保护区、活动案卷和冻结凭据占用，并给出质量预算。

## 构建、运行与测试

在项目根目录执行：

```text
go test ./...
go run ./cmd/server -addr=127.0.0.1:19087 -selfcheck
```

直接运行服务时可使用 `-addr=127.0.0.1:<port>` 指定回环地址，也可设置 `PORT` 端口号。默认地址为 `127.0.0.1:19087`。HTTP API 根路径为 `/api/v1`，数据快照默认保存在 `.data`。

## 主要资源

- `POST /api/v1/cores/{coreId}/impact`：预览岩芯约束修订影响。
- `POST /api/v1/cores/{coreId}/revise`、`GET /api/v1/cores/{coreId}/revisions`：提交修订并查询不可变版本。
- `POST /api/v1/cases/{caseId}/revise`、`GET /api/v1/cases/{caseId}/revisions`：完整修订案卷并查询字段级差异。
- `POST /api/v1/cases/{caseId}/precheck`、`GET /api/v1/cases/{caseId}/prechecks/{digest}`：计算、查询预检快照及当前有效性。
- `POST /api/v1/cases/{caseId}/review`、`GET /api/v1/cases/{caseId}/findings`：登记和查询复核问题；关闭入口为 `POST /api/v1/cases/{caseId}/findings/{findingId}/close`。
- `POST /api/v1/cases/{caseId}/authorize`、`GET /api/v1/cases/{caseId}/authorization`：签发和查询授权清单。
- `POST /api/v1/cases/{caseId}/execute`、`GET /api/v1/cases/{caseId}/receipts/{idempotencyKey}`：执行切割和查询幂等回执。
- `POST /api/v1/cases/{caseId}/freeze`、`GET /api/v1/cases/{caseId}/verifications`：核验余样、冻结案卷及查询每次尝试。
- `GET /api/v1/available/{coreId}`、`GET /api/v1/credentials/{credentialId}`：查询占用解释与逐项凭据验真。

所有写入口使用严格 JSON，业务错误统一返回稳定的 `error.code`、中文 `error.message` 和可选 `error.details`。案卷修订需要 `revisionNote`；关闭问题需要当前 `caseRevision`；执行和核验分别需要非空 `idempotencyKey` 与 `verificationKey`。
