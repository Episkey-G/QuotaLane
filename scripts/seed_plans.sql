-- QuotaLane: Seed Plans Data
-- Description: Insert 5 default subscription plans into the plans table

-- Starter Plan
INSERT INTO `plans` (
    `name`,
    `description`,
    `price`,
    `dollar_limit`,
    `duration_days`,
    `rpm_limit`,
    `features`,
    `badge`,
    `status`
) VALUES (
    'Starter',
    '入门套餐，适合个人开发者和小型项目测试使用',
    9.99,
    10.00,
    30,
    100,
    JSON_OBJECT(
        'support_models', JSON_ARRAY('claude-3-haiku', 'gpt-3.5-turbo'),
        'max_api_keys', 2,
        'support_level', 'community',
        'features', JSON_ARRAY('基础模型访问', '社区支持', '每月报表')
    ),
    'starter',
    'active'
);

-- Basic Plan
INSERT INTO `plans` (
    `name`,
    `description`,
    `price`,
    `dollar_limit`,
    `duration_days`,
    `rpm_limit`,
    `features`,
    `badge`,
    `status`
) VALUES (
    'Basic',
    '基础套餐，适合中小企业和个人开发者日常使用',
    29.99,
    50.00,
    30,
    300,
    JSON_OBJECT(
        'support_models', JSON_ARRAY('claude-3-haiku', 'claude-3-sonnet', 'gpt-3.5-turbo', 'gpt-4'),
        'max_api_keys', 5,
        'support_level', 'email',
        'features', JSON_ARRAY('标准模型访问', '邮件支持', '每周报表', '使用分析')
    ),
    'basic',
    'active'
);

-- Professional Plan
INSERT INTO `plans` (
    `name`,
    `description`,
    `price`,
    `dollar_limit`,
    `duration_days`,
    `rpm_limit`,
    `features`,
    `badge`,
    `status`
) VALUES (
    'Professional',
    '专业套餐，适合中型团队和高频使用场景',
    99.99,
    200.00,
    30,
    1000,
    JSON_OBJECT(
        'support_models', JSON_ARRAY('claude-3-haiku', 'claude-3-sonnet', 'claude-3-opus', 'gpt-3.5-turbo', 'gpt-4', 'gpt-4-turbo'),
        'max_api_keys', 15,
        'support_level', 'priority',
        'features', JSON_ARRAY('全模型访问', '优先支持', '实时监控', '自定义限流', 'Webhook通知')
    ),
    'professional',
    'active'
);

-- Flagship Plan
INSERT INTO `plans` (
    `name`,
    `description`,
    `price`,
    `dollar_limit`,
    `duration_days`,
    `rpm_limit`,
    `features`,
    `badge`,
    `status`
) VALUES (
    'Flagship',
    '旗舰套餐，适合大型企业和高并发生产环境',
    299.99,
    800.00,
    30,
    0,
    JSON_OBJECT(
        'support_models', JSON_ARRAY('all'),
        'max_api_keys', 50,
        'support_level', 'dedicated',
        'features', JSON_ARRAY('全模型无限制访问', '专属客户经理', '99.9% SLA', '自定义账户组', '高级监控', '自定义集成')
    ),
    'flagship',
    'active'
);

-- Exclusive Plan
INSERT INTO `plans` (
    `name`,
    `description`,
    `price`,
    `dollar_limit`,
    `duration_days`,
    `rpm_limit`,
    `features`,
    `badge`,
    `status`
) VALUES (
    'Exclusive',
    '尊享套餐，适合超大型企业和战略合作伙伴',
    999.99,
    5000.00,
    30,
    0,
    JSON_OBJECT(
        'support_models', JSON_ARRAY('all'),
        'max_api_keys', -1,
        'support_level', 'exclusive',
        'features', JSON_ARRAY('全模型无限制访问', '专属技术团队', '99.99% SLA', 'VIP优先级', '定制化开发', '私有部署支持', '战略咨询服务')
    ),
    'exclusive',
    'active'
);
