-- Increase encrypted token column sizes to accommodate large JWTs
-- JWT tokens can be very large (2000+ chars), and AES-256-GCM encryption adds significant overhead
-- Original sizes: access_token(1024), refresh_token(1024), id_token(2048)
-- All upgraded to TEXT to handle encrypted JWTs safely
ALTER TABLE `api_accounts`
  MODIFY COLUMN `access_token_encrypted` TEXT NULL COMMENT '加密的访问令牌',
  MODIFY COLUMN `refresh_token_encrypted` TEXT NULL COMMENT '加密的刷新令牌',
  MODIFY COLUMN `id_token_encrypted` TEXT NULL COMMENT '加密的ID令牌';
