package com.anonymous.quickqueagent

import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import com.tencent.mm.opensdk.modelbase.BaseResp
import com.tencent.mm.opensdk.modelmsg.SendAuth
import com.tencent.mm.opensdk.openapi.WXAPIFactory
import java.util.UUID

class QuickQueWeChatAuthModule(
  reactContext: ReactApplicationContext
) : ReactContextBaseJavaModule(reactContext) {

  override fun getName(): String = "QuickQueWeChatAuth"

  @ReactMethod
  fun signIn(appId: String, promise: Promise) {
    val activity = currentActivity
    if (activity == null) {
      promise.reject("WECHAT_SIGN_IN_ACTIVITY_UNAVAILABLE", "Current activity is unavailable")
      return
    }

    val trimmedAppId = appId.trim()
    if (trimmedAppId.isBlank()) {
      promise.reject("WECHAT_SIGN_IN_NOT_CONFIGURED", "WeChat Sign-In is not configured")
      return
    }

    val api = WXAPIFactory.createWXAPI(activity, trimmedAppId, true)
    api.registerApp(trimmedAppId)
    activeAppId = trimmedAppId

    if (!api.isWXAppInstalled) {
      promise.reject("WECHAT_NOT_INSTALLED", "WeChat is not installed")
      return
    }

    val state = UUID.randomUUID().toString()
    if (!setPending(promise, state)) {
      promise.reject("WECHAT_SIGN_IN_IN_PROGRESS", "WeChat Sign-In is already in progress")
      return
    }

    val request = SendAuth.Req().apply {
      scope = "snsapi_userinfo"
      this.state = state
    }

    if (!api.sendReq(request)) {
      clearPending()
      promise.reject("WECHAT_SEND_REQUEST_FAILED", "Unable to send WeChat auth request")
    }
  }

  @ReactMethod
  fun clearAuthState(promise: Promise) {
    clearPending()
    promise.resolve(null)
  }

  companion object {
    private const val DEFAULT_APP_ID = "wx070caa369d489329"
    private val lock = Any()
    private var pendingPromise: Promise? = null
    private var pendingState: String? = null
    @Volatile private var activeAppId: String = DEFAULT_APP_ID

    fun registeredAppId(): String = activeAppId.ifBlank { DEFAULT_APP_ID }

    fun handleResponse(resp: BaseResp) {
      val pending = takePending()
      val promise = pending?.promise ?: return
      val expectedState = pending.state
      if (resp !is SendAuth.Resp) {
        promise.reject("WECHAT_UNSUPPORTED_RESPONSE", "Unsupported WeChat response")
        return
      }

      when (resp.errCode) {
        BaseResp.ErrCode.ERR_OK -> {
          if (!expectedState.isNullOrBlank() && resp.state != expectedState) {
            promise.reject("WECHAT_STATE_MISMATCH", "WeChat auth state mismatch")
            return
          }
          val code = resp.code?.trim().orEmpty()
          if (code.isBlank()) {
            promise.reject("WECHAT_CODE_MISSING", "WeChat did not return an authorization code")
            return
          }
          promise.resolve(code)
        }
        BaseResp.ErrCode.ERR_USER_CANCEL -> {
          promise.reject("WECHAT_SIGN_IN_CANCELLED", "WeChat Sign-In was cancelled")
        }
        BaseResp.ErrCode.ERR_AUTH_DENIED -> {
          promise.reject("WECHAT_AUTH_DENIED", "WeChat authorization was denied")
        }
        else -> {
          val message = resp.errStr?.takeIf { it.isNotBlank() } ?: "WeChat Sign-In failed"
          promise.reject("WECHAT_SIGN_IN_FAILED", message)
        }
      }
    }

    private fun setPending(promise: Promise, state: String): Boolean {
      synchronized(lock) {
        if (pendingPromise != null) {
          return false
        }
        pendingPromise = promise
        pendingState = state
        return true
      }
    }

    private fun clearPending() {
      synchronized(lock) {
        pendingPromise = null
        pendingState = null
      }
    }

    private fun takePending(): PendingAuth? {
      synchronized(lock) {
        val promise = pendingPromise ?: return null
        val state = pendingState
        pendingPromise = null
        pendingState = null
        return PendingAuth(promise, state)
      }
    }

    private data class PendingAuth(
      val promise: Promise,
      val state: String?
    )
  }
}
