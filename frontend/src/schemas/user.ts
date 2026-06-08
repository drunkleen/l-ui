import { z } from 'zod';

export const UserRecordSchema = z.object({
  id: z.number(),
  username: z.string(),
}).loose();

export const UserListSchema = z.array(UserRecordSchema);

export type UserRecord = z.infer<typeof UserRecordSchema>;
