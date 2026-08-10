export type RuleOutput = { type: 'name' | 'category' | 'transfer'; nameOp: 'RENAME' | 'ADD_PREFIX' | 'ADD_SUFFIX'; value: string; categoryId: string; transferAccountId: string };

export function emptyOutput(): RuleOutput {
  return { type: 'name', nameOp: 'RENAME', value: '', categoryId: '', transferAccountId: '' };
}

export function toRuleAction(o: RuleOutput): any {
  if (o.type === 'category') return { setName: '', setNameOp: 'RENAME', setCategoryId: o.categoryId, setTransferAccountId: '' };
  if (o.type === 'transfer') return { setName: '', setNameOp: 'RENAME', setCategoryId: '', setTransferAccountId: o.transferAccountId };
  return { setName: o.value, setNameOp: o.nameOp, setCategoryId: '', setTransferAccountId: '' };
}

export function fromRuleAction(a: any): RuleOutput | null {
  if (a?.setTransferAccountId) return { type: 'transfer', nameOp: 'RENAME', value: '', categoryId: '', transferAccountId: a.setTransferAccountId };
  if (a?.setCategoryId) return { type: 'category', nameOp: 'RENAME', value: '', categoryId: a.setCategoryId, transferAccountId: '' };
  if (a?.setName) return { type: 'name', nameOp: a.setNameOp || 'RENAME', value: a.setName, categoryId: '', transferAccountId: '' };
  return null;
}
