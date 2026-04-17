package io.exra.node

import android.app.Application
import android.util.Log
import com.google.firebase.FirebaseApp
import com.google.firebase.crashlytics.FirebaseCrashlytics

class ExraApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        
        // Initialize Firebase
        try {
            FirebaseApp.initializeApp(this)
            FirebaseCrashlytics.getInstance().setCrashlyticsCollectionEnabled(true)
            Log.i("ExraApplication", "Firebase and Crashlytics initialized")
        } catch (e: Exception) {
            Log.e("ExraApplication", "Failed to initialize Firebase: ${e.message}")
        }
        
        // Log basic build info for debugging
        FirebaseCrashlytics.getInstance().setCustomKey("version", BuildConfig.VERSION_NAME)
        FirebaseCrashlytics.getInstance().setCustomKey("build_type", BuildConfig.BUILD_TYPE)
    }
}
