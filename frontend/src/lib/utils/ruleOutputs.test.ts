import { describe, expect, it } from 'vitest';
import { emptyOutput, fromRuleAction, toRuleAction } from './ruleOutputs';

describe('ruleOutputs', () => {
  it('maps a name output to an RuleAction', () => {
    expect(toRuleAction({ type: 'name', nameOp: 1, value: 'Netflix Subscription', categoryId: '', transferAccountId: '' })).toEqual({
      setName: 'Netflix Subscription', setNameOp: 1, setCategoryId: '', setTransferAccountId: ''
    });
  });

  it('maps a category output to an RuleAction', () => {
    expect(toRuleAction({ type: 'category', nameOp: 0, value: '', categoryId: 'cat-1', transferAccountId: '' })).toEqual({
      setName: '', setNameOp: 0, setCategoryId: 'cat-1', setTransferAccountId: ''
    });
  });

  it('maps a transfer output to an RuleAction', () => {
    expect(toRuleAction({ type: 'transfer', nameOp: 0, value: '', categoryId: '', transferAccountId: 'acc-1' })).toEqual({
      setName: '', setNameOp: 0, setCategoryId: '', setTransferAccountId: 'acc-1'
    });
  });

  it('round-trips name and category from an RuleAction', () => {
    expect(fromRuleAction(toRuleAction(emptyOutput() as any))).toBeNull();
    expect(fromRuleAction({ setName: 'X', setNameOp: 2, setCategoryId: '', setTransferAccountId: '' })).toEqual({ type: 'name', nameOp: 2, value: 'X', categoryId: '', transferAccountId: '' });
    expect(fromRuleAction({ setName: '', setNameOp: 0, setCategoryId: 'c', setTransferAccountId: '' })).toEqual({ type: 'category', nameOp: 0, value: '', categoryId: 'c', transferAccountId: '' });
    expect(fromRuleAction({ setName: '', setNameOp: 0, setCategoryId: '', setTransferAccountId: 'a' })).toEqual({ type: 'transfer', nameOp: 0, value: '', categoryId: '', transferAccountId: 'a' });
  });

  it('returns null when the action is empty', () => {
    expect(fromRuleAction({ setName: '', setNameOp: 0, setCategoryId: '', setTransferAccountId: '' })).toBeNull();
  });
});
