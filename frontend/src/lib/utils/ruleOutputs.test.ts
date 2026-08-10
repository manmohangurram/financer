import { describe, expect, it } from 'vitest';
import { emptyOutput, fromRuleAction, toRuleAction } from './ruleOutputs';

describe('ruleOutputs', () => {
  it('maps a name output to an RuleAction', () => {
    expect(toRuleAction({ type: 'name', nameOp: 'RENAME', value: 'Netflix Subscription', categoryId: '', transferAccountId: '' })).toEqual({
      setName: 'Netflix Subscription', setNameOp: 'RENAME', setCategoryId: '', setTransferAccountId: ''
    });
  });

  it('maps a category output to an RuleAction', () => {
    expect(toRuleAction({ type: 'category', nameOp: 'RENAME', value: '', categoryId: 'cat-1', transferAccountId: '' })).toEqual({
      setName: '', setNameOp: 'RENAME', setCategoryId: 'cat-1', setTransferAccountId: ''
    });
  });

  it('maps a transfer output to an RuleAction', () => {
    expect(toRuleAction({ type: 'transfer', nameOp: 'RENAME', value: '', categoryId: '', transferAccountId: 'acc-1' })).toEqual({
      setName: '', setNameOp: 'RENAME', setCategoryId: '', setTransferAccountId: 'acc-1'
    });
  });

  it('round-trips name and category from an RuleAction', () => {
    expect(fromRuleAction(toRuleAction(emptyOutput() as any))).toBeNull();
    expect(fromRuleAction({ setName: 'X', setNameOp: 'ADD_PREFIX', setCategoryId: '', setTransferAccountId: '' })).toEqual({ type: 'name', nameOp: 'ADD_PREFIX', value: 'X', categoryId: '', transferAccountId: '' });
    expect(fromRuleAction({ setName: '', setNameOp: 'RENAME', setCategoryId: 'c', setTransferAccountId: '' })).toEqual({ type: 'category', nameOp: 'RENAME', value: '', categoryId: 'c', transferAccountId: '' });
    expect(fromRuleAction({ setName: '', setNameOp: 'RENAME', setCategoryId: '', setTransferAccountId: 'a' })).toEqual({ type: 'transfer', nameOp: 'RENAME', value: '', categoryId: '', transferAccountId: 'a' });
  });

  it('returns null when the action is empty', () => {
    expect(fromRuleAction({ setName: '', setNameOp: 'RENAME', setCategoryId: '', setTransferAccountId: '' })).toBeNull();
  });
});
