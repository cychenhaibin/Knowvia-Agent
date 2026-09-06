package com.anonymous.quickqueagent

import android.app.Activity
import android.net.Uri
import android.os.CancellationSignal
import org.json.JSONObject
import androidx.core.content.ContextCompat
import androidx.credentials.ClearCredentialStateRequest
import androidx.credentials.CreateCredentialResponse
import androidx.credentials.CredentialManager
import androidx.credentials.CredentialManagerCallback
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.GetCredentialResponse
import androidx.credentials.exceptions.ClearCredentialException
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException
import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import com.google.android.libraries.identity.googleid.GoogleIdTokenParsingException

class QuickQueGoogleAuthModule(
  reactContext: ReactApplicationContext
) : ReactContextBaseJavaModule(reactContext) {

  override fun getName(): String = "QuickQueGoogleAuth"

  @ReactMethod
  fun signIn(promise: Promise) {
    val activity = currentActivity
    if (activity == null) {
      promise.reject("GOOGLE_SIGN_IN_ACTIVITY_UNAVAILABLE", "Current activity is unavailable")
      return
    }

    val serverClientId = googleWebClientId()
    if (serverClientId == null) {
      promise.reject("GOOGLE_SIGN_IN_NOT_CONFIGURED", "Google Sign-In is not configured")
      return
    }

    val credentialManager = CredentialManager.create(activity)
    val option = GetSignInWithGoogleOption.Builder(serverClientId).build()
    val request = GetCredentialRequest.Builder()
      .addCredentialOption(option)
      .build()

    credentialManager.getCredentialAsync(
      activity,
      request,
      CancellationSignal(),
      ContextCompat.getMainExecutor(activity),
      object : CredentialManagerCallback<GetCredentialResponse, GetCredentialException> {
        override fun onResult(result: GetCredentialResponse) {
          handleSignInResult(result, promise)
        }

        override fun onError(e: GetCredentialException) {
          handleSignInError(e, promise)
        }
      }
    )
  }

  private fun googleWebClientId(): String? {
    return try {
      val config = reactApplicationContext.resources
        .openRawResource(R.raw.google_auth_config)
        .bufferedReader()
        .use { it.readText() }
      JSONObject(config).optString("web_client_id").trim().takeIf { it.isNotEmpty() }
    } catch (_: Exception) {
      null
    }
  }

  @ReactMethod
  fun clearCredentialState(promise: Promise) {
    val activity = currentActivity
    if (activity == null) {
      promise.resolve(null)
      return
    }

    val credentialManager = CredentialManager.create(activity)
    credentialManager.clearCredentialStateAsync(
      ClearCredentialStateRequest(),
      CancellationSignal(),
      ContextCompat.getMainExecutor(activity),
      object : CredentialManagerCallback<Void?, ClearCredentialException> {
        override fun onResult(result: Void?) {
          promise.resolve(null)
        }

        override fun onError(e: ClearCredentialException) {
          promise.reject("GOOGLE_CLEAR_CREDENTIAL_STATE_FAILED", e.message, e)
        }
      }
    )
  }

  private fun handleSignInResult(result: GetCredentialResponse, promise: Promise) {
    val credential = result.credential
    if (credential !is CustomCredential ||
      credential.type != GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL
    ) {
      promise.reject("GOOGLE_SIGN_IN_UNSUPPORTED_CREDENTIAL", "Unsupported Google credential response")
      return
    }

    try {
      val googleCredential = GoogleIdTokenCredential.createFrom(credential.data)
      val payload = Arguments.createMap().apply {
        putString("idToken", googleCredential.idToken)
        putString("email", googleCredential.id)
        putString("displayName", googleCredential.displayName)
        putString("avatarUrl", googleCredential.profilePictureUri?.toString())
      }
      promise.resolve(payload)
    } catch (e: GoogleIdTokenParsingException) {
      promise.reject("GOOGLE_SIGN_IN_PARSE_FAILED", e.message, e)
    }
  }

  private fun handleSignInError(error: GetCredentialException, promise: Promise) {
    if (error is GetCredentialCancellationException) {
      promise.reject("GOOGLE_SIGN_IN_CANCELLED", error.message, error)
      return
    }
    promise.reject("GOOGLE_SIGN_IN_FAILED", error.message, error)
  }
}
