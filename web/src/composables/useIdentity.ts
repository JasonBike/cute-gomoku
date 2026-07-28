import { readonly, reactive } from "vue";
import type { SessionState, UserProfile } from "@/types/user";

export function useIdentity(onNotice: (message: string) => void) {
  const state = reactive({
    user: null as UserProfile | null,
    expiresAt: 0,
    loading: false,
  });

  async function load() {
    if (state.loading) return state.user;
    state.loading = true;
    try {
      const response = await fetch("/api/session", {
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      const result = (await response.json().catch(() => ({}))) as Partial<SessionState> & {
        message?: string;
      };
      if (!response.ok || !result.user) {
        throw new Error(result.message || "暂时无法加载棋手身份");
      }
      state.user = result.user;
      state.expiresAt = Number(result.expiresAt) || 0;
      return state.user;
    } catch (error) {
      onNotice(error instanceof Error ? error.message : "暂时无法加载棋手身份");
      return null;
    } finally {
      state.loading = false;
    }
  }

  async function updateNickname(nickname: string) {
    const response = await fetch("/api/me", {
      method: "PATCH",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ nickname }),
    });
    const result = (await response.json().catch(() => ({}))) as Partial<SessionState> & {
      message?: string;
    };
    if (!response.ok || !result.user) {
      throw new Error(result.message || "昵称保存失败");
    }
    state.user = result.user;
    state.expiresAt = Number(result.expiresAt) || state.expiresAt;
    return state.user;
  }

  return {
    state: readonly(state),
    load,
    updateNickname,
  };
}
