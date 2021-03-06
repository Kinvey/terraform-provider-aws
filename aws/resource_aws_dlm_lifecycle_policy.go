package aws

import (
	"fmt"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dlm"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/keyvaluetags"
)

func resourceAwsDlmLifecyclePolicy() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsDlmLifecyclePolicyCreate,
		Read:   resourceAwsDlmLifecyclePolicyRead,
		Update: resourceAwsDlmLifecyclePolicyUpdate,
		Delete: resourceAwsDlmLifecyclePolicyDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile("^[0-9A-Za-z _-]+$"), "see https://docs.aws.amazon.com/cli/latest/reference/dlm/create-lifecycle-policy.html"),
				//	TODO: https://docs.aws.amazon.com/dlm/latest/APIReference/API_LifecyclePolicy.html#dlm-Type-LifecyclePolicy-Description says it has max length of 500 but doesn't mention the regex but SDK and CLI docs only mention the regex and not max length. Check this
			},
			"execution_role_arn": {
				// TODO: Make this not required and if it's not provided then use the default service role, creating it if necessary
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateArn,
			},
			"policy_details": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"resource_types": {
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"schedule": {
							Type:     schema.TypeList,
							Required: true,
							MinItems: 1,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"copy_tags": {
										Type:     schema.TypeBool,
										Optional: true,
										Computed: true,
										ForceNew: true,
									},
									"create_rule": {
										Type:     schema.TypeList,
										Required: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"cron_expression": {
													Type:          schema.TypeString,
													Optional:      true,
													ConflictsWith: []string{"policy_details.0.schedule.0.create_rule.0.interval", "policy_details.0.schedule.0.create_rule.0.interval_unit", "policy_details.0.schedule.0.create_rule.0.times"},
													ValidateFunc:  validation.StringMatch(regexp.MustCompile(`^cron\([^\n]{11,100}\)$`), "see https://docs.aws.amazon.com/dlm/latest/APIReference/API_CreateRule.html#dlm-Type-CreateRule-CronExpression"),
												},
												"interval": {
													Type:          schema.TypeInt,
													Optional:      true,
													ConflictsWith: []string{"policy_details.0.schedule.0.create_rule.0.cron_expression"},
													ValidateFunc:  validation.IntInSlice([]int{1, 2, 3, 4, 6, 8, 12, 24}),
												},
												"interval_unit": {
													Type:          schema.TypeString,
													Optional:      true,
													ConflictsWith: []string{"policy_details.0.schedule.0.create_rule.0.cron_expression"},
													ValidateFunc: validation.StringInSlice([]string{
														dlm.IntervalUnitValuesHours,
													}, false),
												},
												"times": {
													Type:          schema.TypeList,
													Optional:      true,
													Computed:      true,
													ConflictsWith: []string{"policy_details.0.schedule.0.create_rule.0.cron_expression"},
													MaxItems:      1,
													Elem: &schema.Schema{
														Type:         schema.TypeString,
														ValidateFunc: validation.StringMatch(regexp.MustCompile("^(0[0-9]|1[0-9]|2[0-3]):[0-5][0-9]$"), "see https://docs.aws.amazon.com/dlm/latest/APIReference/API_CreateRule.html#dlm-Type-CreateRule-Times"),
													},
												},
											},
										},
									},
									"cross_region_copy_rule": {
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"cmk_arn": {
													Type:         schema.TypeString,
													Optional:     true,
													Default:      "",
													ValidateFunc: validateArn,
												},
												"copy_tags": {
													Type:     schema.TypeBool,
													Optional: true,
												},
												"encrypted": {
													Type:     schema.TypeBool,
													Required: true,
												},
												"retain_rule": {
													Type:     schema.TypeList,
													Required: true,
													MaxItems: 2,
													Elem: &schema.Resource{
														Schema: map[string]*schema.Schema{
															"interval": {
																Type:     schema.TypeInt,
																Required: true,
															},
															"interval_unit": {
																Type:     schema.TypeString,
																Required: true,
																ValidateFunc: validation.StringInSlice([]string{
																	dlm.RetentionIntervalUnitValuesDays,
																	dlm.RetentionIntervalUnitValuesWeeks,
																	dlm.RetentionIntervalUnitValuesMonths,
																	dlm.RetentionIntervalUnitValuesYears,
																}, false),
															},
														},
													},
												},
												"target_region": {
													Type:     schema.TypeString,
													Required: true,
												},
											},
										},
									},
									"name": {
										Type:         schema.TypeString,
										Required:     true,
										ValidateFunc: validation.StringLenBetween(0, 500),
									},
									"retain_rule": {
										Type:     schema.TypeList,
										Required: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"count": {
													Type:         schema.TypeInt,
													Required:     true,
													ValidateFunc: validation.IntBetween(1, 1000),
												},
											},
										},
									},
									"tags_to_add": {
										Type:     schema.TypeMap,
										Optional: true,
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
								},
							},
						},
						"target_tags": {
							Type:     schema.TypeMap,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"state": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  dlm.SettablePolicyStateValuesEnabled,
				ValidateFunc: validation.StringInSlice([]string{
					dlm.SettablePolicyStateValuesDisabled,
					dlm.SettablePolicyStateValuesEnabled,
				}, false),
			},
			"tags": tagsSchema(),
		},
	}
}

func resourceAwsDlmLifecyclePolicyCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dlmconn

	input := dlm.CreateLifecyclePolicyInput{
		Description:      aws.String(d.Get("description").(string)),
		ExecutionRoleArn: aws.String(d.Get("execution_role_arn").(string)),
		PolicyDetails:    expandDlmPolicyDetails(d.Get("policy_details").([]interface{})),
		State:            aws.String(d.Get("state").(string)),
	}

	if v := d.Get("tags").(map[string]interface{}); len(v) > 0 {
		input.Tags = keyvaluetags.New(v).IgnoreAws().DlmTags()
	}

	log.Printf("[INFO] Creating DLM lifecycle policy: %s", input)
	out, err := conn.CreateLifecyclePolicy(&input)
	if err != nil {
		return fmt.Errorf("error creating DLM Lifecycle Policy: %s", err)
	}

	d.SetId(*out.PolicyId)

	return resourceAwsDlmLifecyclePolicyRead(d, meta)
}

func resourceAwsDlmLifecyclePolicyRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dlmconn
	ignoreTagsConfig := meta.(*AWSClient).IgnoreTagsConfig

	log.Printf("[INFO] Reading DLM lifecycle policy: %s", d.Id())
	out, err := conn.GetLifecyclePolicy(&dlm.GetLifecyclePolicyInput{
		PolicyId: aws.String(d.Id()),
	})

	if isAWSErr(err, dlm.ErrCodeResourceNotFoundException, "") {
		log.Printf("[WARN] DLM Lifecycle Policy (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading DLM Lifecycle Policy (%s): %s", d.Id(), err)
	}

	d.Set("arn", out.Policy.PolicyArn)
	d.Set("description", out.Policy.Description)
	d.Set("execution_role_arn", out.Policy.ExecutionRoleArn)
	d.Set("state", out.Policy.State)
	if err := d.Set("policy_details", flattenDlmPolicyDetails(out.Policy.PolicyDetails)); err != nil {
		return fmt.Errorf("error setting policy details %s", err)
	}

	if err := d.Set("tags", keyvaluetags.DlmKeyValueTags(out.Policy.Tags).IgnoreAws().IgnoreConfig(ignoreTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %s", err)
	}

	return nil
}

func resourceAwsDlmLifecyclePolicyUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dlmconn

	input := dlm.UpdateLifecyclePolicyInput{
		PolicyId: aws.String(d.Id()),
	}
	updateLifecyclePolicy := false

	if d.HasChange("description") {
		input.Description = aws.String(d.Get("description").(string))
		updateLifecyclePolicy = true
	}
	if d.HasChange("execution_role_arn") {
		input.ExecutionRoleArn = aws.String(d.Get("execution_role_arn").(string))
		updateLifecyclePolicy = true
	}
	if d.HasChange("state") {
		input.State = aws.String(d.Get("state").(string))
		updateLifecyclePolicy = true
	}
	if d.HasChange("policy_details") {
		input.PolicyDetails = expandDlmPolicyDetails(d.Get("policy_details").([]interface{}))
		updateLifecyclePolicy = true
	}

	if updateLifecyclePolicy {
		log.Printf("[INFO] Updating lifecycle policy %s", d.Id())
		_, err := conn.UpdateLifecyclePolicy(&input)
		if err != nil {
			return fmt.Errorf("error updating DLM Lifecycle Policy (%s): %s", d.Id(), err)
		}
	}

	if d.HasChange("tags") {
		o, n := d.GetChange("tags")
		if err := keyvaluetags.DlmUpdateTags(conn, d.Get("arn").(string), o, n); err != nil {
			return fmt.Errorf("error updating tags: %s", err)
		}
	}

	return resourceAwsDlmLifecyclePolicyRead(d, meta)
}

func resourceAwsDlmLifecyclePolicyDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dlmconn

	log.Printf("[INFO] Deleting DLM lifecycle policy: %s", d.Id())
	_, err := conn.DeleteLifecyclePolicy(&dlm.DeleteLifecyclePolicyInput{
		PolicyId: aws.String(d.Id()),
	})
	if err != nil {
		return fmt.Errorf("error deleting DLM Lifecycle Policy (%s): %s", d.Id(), err)
	}

	return nil
}

func expandDlmPolicyDetails(cfg []interface{}) *dlm.PolicyDetails {
	if len(cfg) == 0 || cfg[0] == nil {
		return nil
	}

	policyDetails := &dlm.PolicyDetails{}
	m := cfg[0].(map[string]interface{})
	if v, ok := m["resource_types"]; ok {
		policyDetails.ResourceTypes = expandStringList(v.([]interface{}))
	}
	if v, ok := m["schedule"]; ok {
		policyDetails.Schedules = expandDlmSchedules(v.([]interface{}))
	}
	if v, ok := m["target_tags"]; ok {
		policyDetails.TargetTags = expandDlmTags(v.(map[string]interface{}))
	}

	return policyDetails
}

func flattenDlmPolicyDetails(policyDetails *dlm.PolicyDetails) []map[string]interface{} {
	result := make(map[string]interface{})
	result["resource_types"] = flattenStringList(policyDetails.ResourceTypes)
	result["schedule"] = flattenDlmSchedules(policyDetails.Schedules)
	result["target_tags"] = flattenDlmTags(policyDetails.TargetTags)

	return []map[string]interface{}{result}
}

func expandDlmSchedules(cfg []interface{}) []*dlm.Schedule {
	schedules := make([]*dlm.Schedule, len(cfg))
	for i, c := range cfg {
		schedule := &dlm.Schedule{}
		m := c.(map[string]interface{})
		if v, ok := m["copy_tags"]; ok {
			schedule.CopyTags = aws.Bool(v.(bool))
		}
		if v, ok := m["create_rule"]; ok {
			schedule.CreateRule = expandDlmCreateRule(v.([]interface{}))
		}
		if v, ok := m["cross_region_copy_rule"]; ok {
			schedule.CrossRegionCopyRules = expandDlmCrossRegionCopyRules(v.([]interface{}))
		}
		if v, ok := m["name"]; ok {
			schedule.Name = aws.String(v.(string))
		}
		if v, ok := m["retain_rule"]; ok {
			schedule.RetainRule = expandDlmRetainRule(v.([]interface{}))
		}
		if v, ok := m["tags_to_add"]; ok {
			schedule.TagsToAdd = expandDlmTags(v.(map[string]interface{}))
		}
		schedules[i] = schedule
	}

	return schedules
}

func flattenDlmSchedules(schedules []*dlm.Schedule) []map[string]interface{} {
	result := make([]map[string]interface{}, len(schedules))
	for i, s := range schedules {
		m := make(map[string]interface{})
		m["copy_tags"] = aws.BoolValue(s.CopyTags)
		m["create_rule"] = flattenDlmCreateRule(s.CreateRule)
		m["cross_region_copy_rule"] = flattenDlmCrossRegionCopyRules(s.CrossRegionCopyRules)
		m["name"] = aws.StringValue(s.Name)
		m["retain_rule"] = flattenDlmRetainRule(s.RetainRule)
		m["tags_to_add"] = flattenDlmTags(s.TagsToAdd)
		result[i] = m
	}

	return result
}

func expandDlmCrossRegionCopyRules(cfg []interface{}) []*dlm.CrossRegionCopyRule {
	cross_region_copy_rules := make([]*dlm.CrossRegionCopyRule, len(cfg))
	for i, c := range cfg {
		cross_region_copy_rule := &dlm.CrossRegionCopyRule{}
		m := c.(map[string]interface{})
		if v, ok := m["cmk_arn"]; ok {
			if m["cmk_arn"] != "" {
				cross_region_copy_rule.CmkArn = aws.String(v.(string))
			}
		}
		if v, ok := m["copy_tags"]; ok {
			cross_region_copy_rule.CopyTags = aws.Bool(v.(bool))
		}
		if v, ok := m["encrypted"]; ok {
			cross_region_copy_rule.Encrypted = aws.Bool(v.(bool))
		}
		if v, ok := m["retain_rule"]; ok {
			cross_region_copy_rule.RetainRule = expandDlmCrossRegionCopyRetainRule(v.([]interface{}))
		}
		if v, ok := m["target_region"]; ok {
			cross_region_copy_rule.TargetRegion = aws.String(v.(string))
		}
		cross_region_copy_rules[i] = cross_region_copy_rule
	}

	return cross_region_copy_rules
}

func flattenDlmCrossRegionCopyRules(crossregioncopyrules []*dlm.CrossRegionCopyRule) []map[string]interface{} {
	result := make([]map[string]interface{}, len(crossregioncopyrules))
	for i, s := range crossregioncopyrules {
		m := make(map[string]interface{})
		if m["cmk_arn"] != "" {
			m["cmk_arn"] = aws.StringValue(s.CmkArn)
		}
		m["copy_tags"] = aws.BoolValue(s.CopyTags)
		m["encrypted"] = aws.BoolValue(s.Encrypted)
		m["retain_rule"] = flattenDlmCrossRegionCopyRetainRule(s.RetainRule)
		m["target_region"] = aws.StringValue(s.TargetRegion)
		result[i] = m
	}

	return result
}

func expandDlmCrossRegionCopyRetainRule(cfg []interface{}) *dlm.CrossRegionCopyRetainRule {
	if len(cfg) == 0 || cfg[0] == nil {
		return nil
	}
	m := cfg[0].(map[string]interface{})
	return &dlm.CrossRegionCopyRetainRule{
		Interval:     aws.Int64(int64(m["interval"].(int))),
		IntervalUnit: aws.String(m["interval_unit"].(string)),
	}
}

func flattenDlmCrossRegionCopyRetainRule(crossRegionCopyRetainRule *dlm.CrossRegionCopyRetainRule) []map[string]interface{} {
	result := make(map[string]interface{})
	result["interval"] = aws.Int64Value(crossRegionCopyRetainRule.Interval)
	result["interval_unit"] = aws.StringValue(crossRegionCopyRetainRule.IntervalUnit)

	return []map[string]interface{}{result}
}

func expandDlmCreateRule(cfg []interface{}) *dlm.CreateRule {
	if len(cfg) == 0 || cfg[0] == nil {
		return nil
	}
	c := cfg[0].(map[string]interface{})
	createRule := &dlm.CreateRule{}

	cronExpression := c["cron_expression"]
	// Prioritize cron_expression
	if cronExpression != "" {
		createRule.CronExpression = aws.String(cronExpression.(string))
	} else {
		createRule.Interval = aws.Int64(int64(c["interval"].(int)))
		createRule.IntervalUnit = aws.String(c["interval_unit"].(string))
		if v, ok := c["times"]; ok {
			createRule.Times = expandStringList(v.([]interface{}))
		}
	}

	return createRule
}

func flattenDlmCreateRule(createRule *dlm.CreateRule) []map[string]interface{} {
	if createRule == nil {
		return []map[string]interface{}{}
	}

	result := make(map[string]interface{})

	cronExpression := aws.StringValue(createRule.CronExpression)
	// Prioritize cron_expression
	if cronExpression != "" {
		result["cron_expression"] = cronExpression
	} else {
		result["interval"] = aws.Int64Value(createRule.Interval)
		result["interval_unit"] = aws.StringValue(createRule.IntervalUnit)
		result["times"] = flattenStringList(createRule.Times)
	}

	return []map[string]interface{}{result}
}

func expandDlmRetainRule(cfg []interface{}) *dlm.RetainRule {
	if len(cfg) == 0 || cfg[0] == nil {
		return nil
	}
	m := cfg[0].(map[string]interface{})
	return &dlm.RetainRule{
		Count: aws.Int64(int64(m["count"].(int))),
	}
}

func flattenDlmRetainRule(retainRule *dlm.RetainRule) []map[string]interface{} {
	result := make(map[string]interface{})
	result["count"] = aws.Int64Value(retainRule.Count)

	return []map[string]interface{}{result}
}

func expandDlmTags(m map[string]interface{}) []*dlm.Tag {
	var result []*dlm.Tag
	for k, v := range m {
		result = append(result, &dlm.Tag{
			Key:   aws.String(k),
			Value: aws.String(v.(string)),
		})
	}

	return result
}

func flattenDlmTags(tags []*dlm.Tag) map[string]string {
	result := make(map[string]string)
	for _, t := range tags {
		result[aws.StringValue(t.Key)] = aws.StringValue(t.Value)
	}

	return result
}
