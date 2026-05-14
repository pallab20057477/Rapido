-- Migration: Fix GoogleID unique index issue
-- Changes unique index to regular index to allow multiple users with empty GoogleID

-- Drop the old unique index if it exists
DROP INDEX IF EXISTS "idx_public_users_google_id";

-- Create a new regular (non-unique) index
CREATE INDEX IF NOT EXISTS "idx_public_users_google_id" ON "public_users"("google_id");

-- Alternative: If the table is named 'public_users' instead of 'users'
DROP INDEX IF EXISTS "idx_public_users_google_id";
CREATE INDEX IF NOT EXISTS "idx_public_users_google_id_nonunique" ON "public_users"("google_id");
