import { describe, it, expect } from 'vitest';
import { getTagType, getStateLabel } from '../../composables/statusUtils';
describe('StatusBadge 逻辑', () => {
    describe('getTagType', () => {
        it('running → success', () => {
            expect(getTagType('running')).toBe('success');
        });
        it('error → error', () => {
            expect(getTagType('error')).toBe('error');
        });
        it('stopped → default', () => {
            expect(getTagType('stopped')).toBe('default');
        });
        it('starting → warning', () => {
            expect(getTagType('starting')).toBe('warning');
        });
        it('stopping → warning', () => {
            expect(getTagType('stopping')).toBe('warning');
        });
    });
    describe('getStateLabel', () => {
        it('running → 运行中', () => {
            expect(getStateLabel('running')).toBe('运行中');
        });
        it('stopped → 已停止', () => {
            expect(getStateLabel('stopped')).toBe('已停止');
        });
        it('error → 错误', () => {
            expect(getStateLabel('error')).toBe('错误');
        });
        it('starting → 启动中', () => {
            expect(getStateLabel('starting')).toBe('启动中');
        });
        it('stopping → 停止中', () => {
            expect(getStateLabel('stopping')).toBe('停止中');
        });
    });
});
