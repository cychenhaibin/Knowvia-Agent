package com.anonymous.quickqueagent.wxapi

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import com.anonymous.quickqueagent.QuickQueWeChatAuthModule
import com.tencent.mm.opensdk.modelbase.BaseReq
import com.tencent.mm.opensdk.modelbase.BaseResp
import com.tencent.mm.opensdk.openapi.IWXAPI
import com.tencent.mm.opensdk.openapi.IWXAPIEventHandler
import com.tencent.mm.opensdk.openapi.WXAPIFactory

class WXEntryActivity : Activity(), IWXAPIEventHandler {
  private lateinit var api: IWXAPI

  override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)
    api = WXAPIFactory.createWXAPI(this, QuickQueWeChatAuthModule.registeredAppId(), true)
    api.handleIntent(intent, this)
  }

  override fun onNewIntent(intent: Intent) {
    super.onNewIntent(intent)
    setIntent(intent)
    api.handleIntent(intent, this)
  }

  override fun onReq(req: BaseReq) = Unit

  override fun onResp(resp: BaseResp) {
    QuickQueWeChatAuthModule.handleResponse(resp)
    finish()
  }
}
