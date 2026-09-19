# 教师课程改革证据服务

保管十个国家高校教师教育课程的**原始版本材料**、专家提交的**跨体系映射**、
映射所支撑的**政策建议谱系**，以及改革试点按**统一口径版本**绑定的方案与阶段结果。

服务要防止四类典型误判：

1. 把不同制度下名称相近的核心素养、实习与持续专业发展（CPD）当成同一概念；
2. “AI 独立设课还是融入现有课程”仅凭课程名称制造虚假多数意见；
3. 映射修订后，旧版曾支撑过哪些政策建议无从追溯；
4. 试点样本缺失或统计定义改变后，仍自动生成成效结论。

## 运行

```bash
go run ./cmd/server                 # 默认 :8080，数据文件 data/evidence.json，首次启动写入十国演示数据
go run ./cmd/server -demo=false     # 不写演示数据
go run ./cmd/server -data /tmp/x.json -addr :9090
go test ./...                       # 全部检查
```

所有数据在每次变更后以 JSON 快照原子落盘（先写 `.tmp` 再 rename），重启不丢审批状态。

## 领域模型（`internal/evidence`）

| 对象 | 关键约束 |
| --- | --- |
| 制度背景 `SystemContext` | 按国家登记，先有制度才能挂院校与框架 |
| 院校 `Institution` | 必须隶属某制度 |
| 能力框架 `Framework` | 按版本登记，素养领域保留各制度自己的命名与 key |
| 课程版本 `CourseVersion` | 以 `课程ID@版本` 为不可变引用；学分/学分单位/先修关系/出处/生效日期；AI 开设方式显式区分为 `standalone`（独立设课）、`integrated`（融入，须注明融入的课程）、`none`；实践环节单独记录周数/学时/是否导师制；引用的框架必须属于同一制度，先修课程版本必须已存在 |
| 持续专业发展 `CPDProgram` | 独立类别，区分职前/入职辅导/在职（`preservice/induction/inservice`），**不与职前课程混类比较** |
| 出处 `Source` | 课程与 CPD 必须引用已登记出处 |
| 映射 `Mapping` | 实例 ID 为 `逻辑ID@版本`；携带两端引用、置信度（0–1）、适用范围、理据、提交专家、异议与审批记录 |

映射状态：`proposed`（待审）→ `approved` / `rejected`；提交修订版后旧版变 `superseded`，
**新版必须重新走审批**。映射两端必须：属于不同制度、且是同类对象
（素养↔素养、课程↔课程、实践↔实践、CPD↔CPD）。驳回必须附书面理由。

| 对象 | 关键约束 |
| --- | --- |
| 政策建议 `PolicyRecommendation` | 创建时只能引用**获批且未被替代**的映射，并把映射**完整内容冻结进快照**；之后映射再怎么修订，旧建议的依据仍可原样回溯 |
| 统计口径 `Calibration` | 实例 ID `逻辑ID@版本`，冻结课程版本引用、框架引用、每项指标的统计定义、预期院校与教师群体、最低应答率；同逻辑 ID 重登记即产生新版本，旧版标记被谁替代，并带内容指纹 |
| 试点 `Pilot` | 方案、参与院校、教师群体、观察指标在建点时绑定一个**未被替代**的口径版本；指标必须取自口径定义，院校/群体必须在口径样本范围内 |
| 阶段结果 `PilotStageResult` | 上报须过三道闸门（见下）；系统本身**从不自动生成结论**，结论只能在过闸后由人显式登记 continue/adjust/stop，并记录所依据的口径版本 |
| 课程调整 `CourseAdjustment` | 关联政策建议与试点，经证据链接口回溯 |

### 阶段上报的三道闸门（任一不过即拒绝写入，HTTP 422）

1. **定义一致**：口径内指标一个不能少，观察值不得夹带口径外指标；
2. **样本完整**：口径声明的预期院校与教师群体不得出现在缺失名单；
3. **应答率达标**：各校应答率不得低于口径下限。

口径一旦被新版本替代，旧试点的任何上报与结论操作都被拒绝（`ErrCalibrationChanged`），
历史结论只能按其当时的口径版本解读。

## 证据链

`GET /v1/adjustments/{id}/evidence-chain` 对一次课程调整给出：

- `comparable_materials`：**当前真正可比**的材料——建议快照中、现在仍获批未被替代的映射
  （含置信度、适用范围、是否有未撤回异议、反向支撑它的建议清单）；
- `gaps`：**尚无共识的部分**——待审/驳回/被替代的映射、旧快照已落后于新版的说明
  （含“曾支撑哪条建议、同概念最新版本是什么状态”）、以及没有任何获批映射覆盖的制度；
- `recommendations`：每条建议的冻结快照与新鲜度检查（依据是否已被修订）；
- `pilot_stages`：每个阶段所用口径版本、指纹、观察值、闸门是否通过、continue/adjust/stop
  结论与作者——说明试点数据**为何**支持继续、改动或停止。

## HTTP 接口（`/v1`）

```
POST   /systems | /institutions | /sources | /frameworks | /courses | /cpds
GET    /courses | /courses/{ref}

POST   /mappings                         提交跨体系映射（proposed）
POST   /mappings/{id}/revise             提交修订版（旧版 superseded，新版重新审批）
POST   /mappings/{id}/objections         登记异议
POST   /mappings/{id}/reviews            审批（approved/rejected，驳回须 comment）
GET    /mappings/{id}
GET    /mapping-lineages/{logicalID}     查看同一概念的全部版本
GET    /comparisons/{kind}               正式比较：只含 approved 且未替代的映射
                                           kind ∈ competency|course|practice|cpd

POST   /recommendations                  立据（映射必须获批），证据即时冻结
GET    /recommendations/{id}
GET    /recommendations/{id}/freshness   依据新旧检查
GET    /backed-recommendations?mapping_id=M-1@1
                                         某映射版本（含旧版）曾支撑过哪些建议

POST   /calibrations                     登记/修订统计口径
POST   /pilots                           按口径版本建点
GET    /pilots/{id}
POST   /pilots/{id}/stages               阶段上报（过闸门才写入）
POST   /pilots/{id}/stages/{stage}/conclusion  显式登记 continue/adjust/stop

POST   /adjustments                      提出课程调整
GET    /adjustments/{id}/evidence-chain  证据链
```

错误以 `{"error": "..."}` 返回：400 参数不合法，404 不存在，409 冲突，
422 违反业务闸门（未获批、跨制度、样本不完整、口径已变更等）。

## 最小工作流示例

```bash
# 1. 登记制度、框架、院校与课程版本（AI 独立设课 vs 融入式，结构上就不同）
# 2. 专家提交映射，附置信度与适用范围
curl -X POST localhost:8080/v1/mappings -d '{
  "kind":"competency",
  "source":{"system_id":"SYS-CN","kind":"framework_domain","ref_id":"FW-CN-2#tech"},
  "target":{"system_id":"SYS-FI","kind":"framework_domain","ref_id":"FW-FI-1#digital"},
  "confidence":0.72,"applicability":"仅限职前数字素养总体要求","expert_id":"expert-wang"}'
# -> M-1@1, status=proposed

# 3. 未审批时比较为空（不会出现虚假多数）；获批后才进入比较
curl -X POST localhost:8080/v1/mappings/M-1@1/reviews -d \
  '{"reviewer_id":"board","decision":"approved","comment":"按声明范围通过"}'
curl localhost:8080/v1/comparisons/competency

# 4. 立政策建议（冻结快照）；日后修订映射，仍可回溯旧版支撑过该建议
curl "localhost:8080/v1/backed-recommendations?mapping_id=M-1@1"

# 5. 登记口径 v1、建点、过闸门上报、再显式下结论；
#    样本缺失/指标漂移/口径升版都会被 422 拒绝
# 6. 决策者提调整后沿证据链查看
curl localhost:8080/v1/adjustments/ADJ-1/evidence-chain
```

`fixtures/curriculum.json` 保留课程方案与能力框架最小关联的静态示例；
运行时的完整相互关联状态见首次启动生成的演示数据（中芬数字素养映射 v1→v2、
待审的 AI 课程映射、日本入职研修 CPD、已绑定口径并得出 continue 的试点）。

## 代码结构

```
cmd/server/main.go            启动参数与装配
internal/evidence/model.go    领域模型
internal/evidence/registry.go 制度/院校/框架/课程/CPD 登记与校验
internal/evidence/mappings.go 映射提交、异议、审批、修订谱系、正式比较
internal/evidence/recommendations.go 政策建议证据快照、回溯与新鲜度
internal/evidence/pilots.go   口径版本、试点绑定、样本与定义闸门、结论登记
internal/evidence/chain.go    课程调整证据链聚合
internal/evidence/store.go    内存存储 + JSON 快照持久化
internal/evidence/seed.go     十国演示数据
internal/api/                 HTTP 路由与处理器
```
