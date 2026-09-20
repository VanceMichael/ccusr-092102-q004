# 教师课程改革证据服务

为教师教育改革工作组保管十国高校的**原始证据**，并对跨体系比较、口径版本与改革试点实施强约束，避免仅凭课程名称把不同制度下的核心素养、实习与持续专业发展混为一谈，也避免"AI 独立设课还是融入课程"得出虚假多数。

运行服务：

```bash
go run ./cmd/server
```

运行检查：

```bash
go test ./...
```

## 设计原则

1. **原始材料不可变**：国家制度背景、院校、能力框架版本、课程（原始课程代码、原文名称、原始学分制、先修关系、实践环节、CPD 标记、主题开设方式与研究出处）登记后不可修改；方案修订须以新 `program_version` 和新 ID 重新登记。
2. **映射是专家意见，不是事实**：每条跨体系映射记录规范概念锚点、关系（equivalent/narrower/broader/related）、置信度、适用范围、判定依据、异议与审批状态。状态流转为 `submitted → approved/rejected`；修订生成新版本并把旧版置为 `superseded`，通过 `revision_of` 保留修订链。
3. **未批准不进入正式比较**：只有 `approved` 映射可进入口径和比较；`submitted/rejected/superseded` 一律排除。未提交获批映射的国家在比较结果中显式列入 `no_consensus_countries`，不被静默忽略。
4. **开设方式按国家计票且必须有出处**：`standalone`（独立设课）/ `embedded`（融入）/ `mixed`（一国两种并存）/ `unknown`，一国一票，投票证据逐课附带原始出处；没有任何已批准映射时拒绝生成比较。
5. **口径版本（caliber）冻结比较基准**：口径同时冻结一组已批准映射版本与一套指标统计定义（含定义指纹）。指标定义改变必须建立新口径，不可覆盖。
6. **试点与口径强绑定**：试点的方案、参与院校、教师群体、观察指标、阶段结果全部挂在同一口径上；指标只能取自该口径。
7. **阻断项存在时禁止成效结论**：样本缺失、未上报、统计定义指纹漂移都会记为阶段 `blockers`；有阻断项时不能附结论，系统也**从不自动生成** continue/modify/stop——结论必须由专家显式记录（记录人、决策、理由）。
8. **证据链可回溯**：政策建议精确引用口径、试点阶段与映射版本（即使映射日后被修订替代，旧版仍保留并在证据链中标注当前状态）。一次课程调整提案只能引用同一口径下的建议；`GET /v1/proposals/{id}/evidence-chain` 沿 提案 → 建议 → 映射版本/阶段结论 → 口径锚定的正式比较 回溯，并额外列出关联试点下所有存在阻断项的阶段。

## API

| 方法与路径 | 作用 |
| --- | --- |
| `POST /v1/countries` | 登记国家/地区制度背景 |
| `POST /v1/institutions` | 登记院校 |
| `POST /v1/frameworks` | 登记能力框架不可变版本（含条目原文） |
| `POST /v1/courses` / `GET /v1/courses/{id}` | 登记/读取原始课程 |
| `POST /v1/mappings` | 提交跨体系映射（状态为 submitted） |
| `GET /v1/mappings/{id}` | 读取映射任意版本 |
| `POST /v1/mappings/{id}/reviews` | 审批/驳回 |
| `POST /v1/mappings/{id}/objections` | 登记专家异议 |
| `POST /v1/mappings/{id}/revisions` | 修订（旧版 superseded，新版重新走审批） |
| `GET /v1/comparisons/{concept}` | 正式比较（仅已批准映射；按国家计票） |
| `POST /v1/calibers` / `GET /v1/calibers/{id}` | 创建/读取口径版本 |
| `POST /v1/pilots` / `GET /v1/pilots/{id}` | 创建/读取试点（绑定口径） |
| `POST /v1/pilots/{id}/stages` | 录入阶段结果（执行阻断校验） |
| `POST /v1/recommendations` | 政策建议（引用已结论阶段与口径内映射） |
| `POST /v1/proposals` | 决策者的课程调整提案（同一口径） |
| `GET /v1/proposals/{id}/evidence-chain` | 完整证据链 |

错误状态码：`400` 输入不合法、`404` 不存在、`409` 与不可变记录冲突、`422` 当前状态不允许（如未审批即比较、有阻断项即下结论）。

## 示例流程

```bash
# 1. 登记制度背景、院校、框架与原始课程（出处必填）
# 2. 专家提交映射，委员会审批；有异议可登记
curl -X POST localhost:8080/v1/mappings/M001/reviews \
  -d '{"reviewer":"评审委员会","approve":true}'
# 3. 冻结口径（映射版本 + 指标统计定义），创建绑定口径的试点
# 4. 录入阶段结果：样本缺失/定义漂移会返回 blockers，此时不能下结论
# 5. 完整阶段由专家显式记录 continue/modify/stop
# 6. 形成政策建议与课程调整提案，并沿证据链查看：
curl localhost:8080/v1/proposals/pp1/evidence-chain
```

`fixtures/curriculum.json` 展示课程方案与能力框架版本的最小关联方式；`service_test.go` 与 `server_test.go` 以四国（CN 独立设课 / DE 融入 / FI 两种并存 / BR 无映射）场景覆盖全部上述规则。

## 范围说明

当前为内存存储（重启清空），领域规则与并发安全集中在 `service.go`，HTTP 层只做编解码与状态码映射；接入持久化存储时只需替换 `Service` 的存储实现，规则与接口契约不变。
