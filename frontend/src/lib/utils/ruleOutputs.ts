export type RuleOutput = { type: 'name' | 'category' | 'transfer'; nameOp: number; value: string; categoryId: string; transferAccountId: string };

export function emptyOutput(): RuleOutput {
  return { type: 'name', nameOp: 1, value: '', categoryId: '', transferAccountId: '' };
}

export function toRuleAction(o: RuleOutput): any {
  if (o.type === 'category') return { setName: '', setNameOp: 0, setCategoryId: o.categoryId, setTransferAccountId: '' };
  if (o.type === 'transfer') return { setName: '', setNameOp: 0, setCategoryId: '', setTransferAccountId: o.transferAccountId };
  return { setName: o.value, setNameOp: o.nameOp, setCategoryId: '', setTransferAccountId: '' };
}

export function fromRuleAction(a: any): RuleOutput | null {
  if (a?.setTransferAccountId) return { type: 'transfer', nameOp: 0, value: '', categoryId: '', transferAccountId: a.setTransferAccountId };
  if (a?.setCategoryId) return { type: 'category', nameOp: 0, value: '', categoryId: a.setCategoryId, transferAccountId: '' };
  if (a?.setName) return { type: 'name', nameOp: a.setNameOp || 1, value: a.setName, categoryId: '', transferAccountId: '' };
  return null;
}
