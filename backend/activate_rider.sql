-- Activate the rider account for phone 9876541111
UPDATE public_users 
SET 
    role = 'rider',
    is_active = true,
    name = 'Test Rider 2',
    email = 'rider2@example.com',
    updated_at = NOW()
WHERE phone = '9876541111';

-- Verify the activation
SELECT id, phone, name, email, role, is_active, created_at, updated_at 
FROM public_users 
WHERE phone = '9876541111';

-- This will activate the rider account and show the updated details
