-- Revert encrypted token column sizes back to original VARCHAR lengths
ALTER TABLE `api_accounts`
  MODIFY COLUMN `access_token_encrypted` VARCHAR(1024) NULL COMMENT '加密的访问令牌',
  MODIFY COLUMN `refresh_token_encrypted` VARCHAR(1024) NULL COMMENT '加密的刷新令牌',
  MODIFY COLUMN `id_token_encrypted` VARCHAR(2048) NULL COMMENT '加密的ID令牌';
