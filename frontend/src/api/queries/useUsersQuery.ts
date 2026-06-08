import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import { UserListSchema, type UserRecord } from '@/schemas/user';

async function fetchUsers(): Promise<UserRecord[]> {
  const msg = await HttpUtil.get('/panel/setting/users', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch users');
  const validated = parseMsg(msg, UserListSchema, 'setting/users');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useUsersQuery() {
  return useQuery({
    queryKey: keys.settings.users(),
    queryFn: fetchUsers,
    staleTime: Infinity,
  });
}
