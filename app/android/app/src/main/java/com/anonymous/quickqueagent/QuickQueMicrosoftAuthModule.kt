package com.anonymous.quickqueagent

import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import com.microsoft.identity.client.AcquireTokenParameters
import com.microsoft.identity.client.AuthenticationCallback
import com.microsoft.identity.client.IAccount
import com.microsoft.identity.client.IAuthenticationResult
import com.microsoft.identity.client.IMultipleAccountPublicClientApplication
import com.microsoft.identity.client.IPublicClientApplication
import com.microsoft.identity.client.PublicClientApplication
import com.microsoft.identity.client.exception.MsalException

class QuickQueMicrosoftAuthModule(
  reactContext: ReactApplicationContext
) : ReactContextBaseJavaModule(reactContext) {

  private var publicClientApplication: IMultipleAccountPublicClientApplication? = null

  override fun getName(): String = "QuickQueMicrosoftAuth"

  @ReactMethod
  fun signIn(promise: Promise) {
    val activity = currentActivity
    if (activity == null) {
      promise.reject("MICROSOFT_SIGN_IN_ACTIVITY_UNAVAILABLE", "Current activity is unavailable")
      return
    }

    withApplication(promise) { application ->
      val parameters = AcquireTokenParameters.Builder()
        .startAuthorizationFromActivity(activity)
        .withScopes(listOf("openid", "profile", "email", "User.Read"))
        .withCallback(object : AuthenticationCallback {
          override fun onSuccess(authenticationResult: IAuthenticationResult) {
            val account = authenticationResult.account
            val idToken = account?.idToken?.takeIf { it.isNotBlank() }
            if (idToken == null) {
              promise.reject("MICROSOFT_ID_TOKEN_MISSING", "Microsoft Sign-In did not return an ID token")
              return
            }
            val claims = account.claims.orEmpty()
            val email = when (val value = claims["email"] ?: claims["preferred_username"]) {
              is String -> value
              else -> account.username
            }
            val displayName = when (val value = claims["name"] ?: claims["given_name"]) {
              is String -> value
              else -> account.username
            }
            val payload = Arguments.createMap().apply {
              putString("idToken", idToken)
              putString("email", email)
              putString("displayName", displayName)
            }
            promise.resolve(payload)
          }

          override fun onError(exception: MsalException) {
            promise.reject("MICROSOFT_SIGN_IN_FAILED", exception.message, exception)
          }

          override fun onCancel() {
            promise.reject("MICROSOFT_SIGN_IN_CANCELLED", "Microsoft Sign-In was cancelled")
          }
        })
        .build()

      application.acquireToken(parameters)
    }
  }

  @ReactMethod
  fun clearAccountState(promise: Promise) {
    withApplication(promise) { application ->
      application.getAccounts(object : IPublicClientApplication.LoadAccountsCallback {
        override fun onTaskCompleted(result: MutableList<IAccount>?) {
          val accounts = result.orEmpty()
          if (accounts.isEmpty()) {
            promise.resolve(null)
            return
          }
          removeAccounts(application, accounts, 0, promise)
        }

        override fun onError(exception: MsalException) {
          promise.reject("MICROSOFT_CLEAR_ACCOUNT_STATE_FAILED", exception.message, exception)
        }
      })
    }
  }

  private fun withApplication(
    promise: Promise,
    onCreated: (IMultipleAccountPublicClientApplication) -> Unit
  ) {
    publicClientApplication?.let {
      onCreated(it)
      return
    }

    PublicClientApplication.createMultipleAccountPublicClientApplication(
      reactApplicationContext,
      R.raw.msal_config,
      object : IPublicClientApplication.IMultipleAccountApplicationCreatedListener {
        override fun onCreated(application: IMultipleAccountPublicClientApplication) {
          publicClientApplication = application
          onCreated(application)
        }

        override fun onError(exception: MsalException) {
          promise.reject("MICROSOFT_AUTH_INIT_FAILED", exception.message, exception)
        }
      }
    )
  }

  private fun removeAccounts(
    application: IMultipleAccountPublicClientApplication,
    accounts: List<IAccount>,
    index: Int,
    promise: Promise
  ) {
    if (index >= accounts.size) {
      promise.resolve(null)
      return
    }

    application.removeAccount(
      accounts[index],
      object : IMultipleAccountPublicClientApplication.RemoveAccountCallback {
        override fun onRemoved() {
          removeAccounts(application, accounts, index + 1, promise)
        }

        override fun onError(exception: MsalException) {
          promise.reject("MICROSOFT_CLEAR_ACCOUNT_STATE_FAILED", exception.message, exception)
        }
      }
    )
  }
}
