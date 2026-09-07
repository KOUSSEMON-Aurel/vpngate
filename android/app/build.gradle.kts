plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "net.vpngate.mobile"
    compileSdk = 36

    defaultConfig {
        applicationId = "net.openrelay.vpn"
        minSdk = 26
        targetSdk = 36
        versionCode = 2
        versionName = "1.0.1"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables {
            useSupportLibrary = true
        }
    }

    signingConfigs {
        create("release") {
            enableV1Signing = true
            enableV2Signing = true
            enableV3Signing = true
            val envStoreFile = System.getenv("ANDROID_STORE_FILE")
            val defaultKeyFile = file("release.jks")
            val resolved = when {
                !envStoreFile.isNullOrEmpty() && file(envStoreFile).exists() -> file(envStoreFile)
                !envStoreFile.isNullOrEmpty() && rootProject.file(envStoreFile).exists() -> rootProject.file(envStoreFile)
                defaultKeyFile.exists() -> defaultKeyFile
                else -> null
            }
            if (resolved != null) {
                storeFile = resolved
                storePassword = System.getenv("ANDROID_STORE_PASSWORD") ?: "openrelay123"
                keyAlias = System.getenv("ANDROID_KEY_ALIAS") ?: "openrelay"
                keyPassword = System.getenv("ANDROID_KEY_PASSWORD") ?: "openrelay123"
            }
        }
    }

    splits {
        abi {
            isEnable = true
            reset()
            include("armeabi-v7a", "arm64-v8a", "x86", "x86_64")
            isUniversalApk = true
        }
    }

    buildTypes {
        release {
            val releaseSigning = signingConfigs.getByName("release")
            if (releaseSigning.storeFile != null) {
                signingConfig = releaseSigning
            }
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
            ndk {
                debugSymbolLevel = "FULL"
            }
        }
        debug {
            applicationIdSuffix = ".debug"
            isDebuggable = true
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        compose = true
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
        jniLibs {
            useLegacyPackaging = false
        }
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.activity.compose)

    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.ui)
    implementation(libs.androidx.ui.graphics)
    implementation(libs.androidx.ui.tooling.preview)
    implementation(libs.androidx.material3)
    implementation(libs.androidx.material.icons.extended)
    implementation(libs.androidx.navigation.compose)

    implementation(libs.okhttp)
    implementation(libs.kotlinx.coroutines.android)

    // OpenVPN native engine
    implementation("com.github.nizwar:openvpn_library:b3941ef040")

    // WireGuard / WARP native engine
    implementation("com.wireguard.android:tunnel:1.0.20230706")

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.androidx.test.ext.junit)
}
